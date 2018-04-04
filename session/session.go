package session

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

type Manager struct {
	cookieName  string
	mu          sync.Mutex
	provider    Provider
	maxLifeTime int64
}

type Provider interface {
	SessionInit(sid string) (Session, error)
	SessionRead(sid string) (Session, error)
	SessionDestroy(sid string) error
	SessionGC(maxLifeTime int64)
}

type Session interface {
	Set(key, value interface{}) error
	Get(key interface{}) interface{}
	Delete(key interface{}) error
	SessionID() string
}

func Register(name string, provider Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if provider == nil {
		panic("session: Register provider is nil")
	}
	if _, dup := providers[name]; dup {
		panic("session: Register called twice for provider " + name)
	}
	providers[name] = provider
}

func (manager *Manager) sessionID() string {
	//TODO
	return ""
}

func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (Session, error) {
	cookie, err := r.Cookie(manager.cookieName)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err != nil || cookie.Value == "" {
		sid := manager.sessionID()
		session, err := manager.provider.SessionInit(sid)
		if err != nil {
			return nil, err
		}
		newCookie := &http.Cookie{Name: manager.cookieName, Value: url.QueryEscape(sid),
			Path: "/", HttpOnly: true, MaxAge: int(manager.maxLifeTime)}
		http.SetCookie(w, newCookie)
		return session, nil
	}
	sid, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return nil, err
	}
	session, err := manager.provider.SessionRead(sid)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(manager.cookieName)
	if err != nil {
		return err
	}
	if cookie.Value == "" {
		return fmt.Errorf("cookie.Value is null")
	}
	sid, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if err := manager.provider.SessionDestroy(sid); err != nil {
		return err
	}
	expiration := time.Now()
	newCookie := &http.Cookie{Name: manager.cookieName, Path: "/",
		HttpOnly: true, MaxAge: -1, Expires: expiration}
	http.SetCookie(w, newCookie)
	return nil
}

func (manager *Manager) GC() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.provider.SessionGC(manager.maxLifeTime)
	time.AfterFunc(time.Duration(manager.maxLifeTime), func() { manager.GC() })
}

package session

import (
	"sync"
	"time"

	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
)

func newMemStorage(cf *Config, opts *sessionOptions) (storage, error) {
	st := &mStorage{
		data:   make(map[string]*sessionConnect),
		opts:   opts,
		config: cf,
	}

	util.UntilWithTick(st.gc,
		opts.clock.NewTicker(time.Duration(cf.GcInterval)*time.Second).C(),
		opts.ctx.Done())

	return st, nil
}

type mStorage struct {
	sync.RWMutex
	data map[string]*sessionConnect

	opts   *sessionOptions
	config *Config
}

func (p *mStorage) all() int {
	p.RLock()
	defer p.RUnlock()
	return len(p.data)
}

func (p *mStorage) get(sid string) (*sessionConnect, error) {
	p.RLock()
	defer p.RUnlock()
	s, ok := p.data[sid]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "sid %s is not found", sid)
	}
	return s, nil
}

func (p *mStorage) insert(s *sessionConnect) error {
	p.Lock()
	defer p.Unlock()

	p.data[s.Sid] = s
	return nil
}

func (p *mStorage) del(sid string) error {
	p.Lock()
	defer p.Unlock()

	delete(p.data, sid)
	return nil
}

func (p *mStorage) update(s *sessionConnect) error {
	p.Lock()
	defer p.Unlock()

	p.data[s.Sid] = s
	return nil
}

func (p *mStorage) gc() {
	p.Lock()
	defer p.Unlock()

	expiresAt := p.opts.clock.Now().Unix() - p.config.CookieLifetime
	keys := []string{}
	for k, v := range p.data {
		if v.UpdatedAt < expiresAt {
			keys = append(keys, k)
		}
	}

	for _, k := range keys {
		delete(p.data, k)
	}
}
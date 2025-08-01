package basic_activity

import (
	"math/rand"
	"sync"

	"cosmossdk.io/math"
	"github.com/allora-network/allora-simulator/types"
)

type State struct {
	actors        []*types.Actor
	actorsPerAddr map[string]*types.Actor
	balances      map[string]math.Int

	mutex sync.Mutex
}

func NewState(actors []*types.Actor, balance math.Int) *State {
	balances := make(map[string]math.Int, len(actors))
	perAddr := make(map[string]*types.Actor, len(actors))
	for _, actor := range actors {
		balances[actor.Addr] = balance
		perAddr[actor.Addr] = actor
	}

	return &State{
		actors:        actors,
		actorsPerAddr: perAddr,
		balances:      balances,
		mutex:         sync.Mutex{},
	}
}

func (s *State) getShuffledActors() []*types.Actor {
	actors := make([]*types.Actor, len(s.actors))
	copy(actors, s.actors)
	rand.Shuffle(len(actors), func(i, j int) {
		actors[i], actors[j] = actors[j], actors[i]
	})
	return actors
}

func (s *State) pickRandomActorExcept(addr string) *types.Actor {
	for _, a := range s.getShuffledActors() {
		if a.Addr != addr {
			return a
		}
	}
	return nil
}

func (s *State) decreaseActorBalance(addr string, amount math.Int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if balance, exists := s.balances[addr]; exists {
		if balance.GTE(amount) {
			s.balances[addr] = balance.Sub(amount)
		} else {
			s.balances[addr] = math.ZeroInt()
		}
	}
}

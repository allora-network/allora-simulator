package stress

import (
	"sync"

	"github.com/allora-network/allora-simulator/types"
)

type StressSimulationData struct {
	Faucet                    *types.Actor
	EpochLength               int64
	Actors                    []*types.Actor
	RegisteredWorkersByTopic  map[uint64][]*types.Actor
	RegisteredReputersByTopic map[uint64][]*types.Actor
	FailOnErr                 bool
	Mu                        sync.RWMutex
}

type Registration struct {
	TopicId uint64
	Actor   *types.Actor
}

// Add a worker registration to the simulation data
func (s *StressSimulationData) AddWorkerRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredWorkersByTopic[topicId] = append(s.RegisteredWorkersByTopic[topicId], actor)
}

// Add a reputer registration to the simulation data
func (s *StressSimulationData) AddReputerRegistration(topicId uint64, actor *types.Actor) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.RegisteredReputersByTopic[topicId] = append(s.RegisteredReputersByTopic[topicId], actor)
}

// Get an actor object from an address
func (s *StressSimulationData) GetActorFromAddr(addr string) (*types.Actor, bool) {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	for _, actor := range s.Actors {
		if actor.Addr == addr {
			return actor, true
		}
	}
	return nil, false
}

// Get all workers for a topic
func (s *StressSimulationData) GetWorkersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredWorkersByTopic[topicId]
}

// Get all reputers for a topic
func (s *StressSimulationData) GetReputersForTopic(topicId uint64) []*types.Actor {
	s.Mu.RLock()
	defer s.Mu.RUnlock()
	return s.RegisteredReputersByTopic[topicId]
}

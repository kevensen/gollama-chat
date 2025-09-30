package chat

import (
	"context"
	"fmt"
	"sync"
)

// ModelContextService provides channel-based access to model context sizes
type ModelContextService struct {
	requests chan contextRequest
	shutdown chan struct{}
	done     chan struct{}
	wg       sync.WaitGroup
}

// contextRequest represents a request to the context service
type contextRequest struct {
	operation string // "get", "set", "clear"
	key       string
	value     int
	response  chan contextResponse
}

// contextResponse represents a response from the context service
type contextResponse struct {
	value int
	found bool
	error error
}

// NewModelContextService creates a new channel-based model context service
func NewModelContextService() *ModelContextService {
	service := &ModelContextService{
		requests: make(chan contextRequest, 10), // Small buffer for async operations
		shutdown: make(chan struct{}),
		done:     make(chan struct{}),
	}

	service.wg.Add(1)
	go service.worker()

	return service
}

// worker runs the main service loop
func (s *ModelContextService) worker() {
	defer s.wg.Done()
	defer close(s.done)

	cache := make(map[string]int)

	for {
		select {
		case req := <-s.requests:
			switch req.operation {
			case "get":
				value, found := cache[req.key]
				req.response <- contextResponse{
					value: value,
					found: found,
					error: nil,
				}

			case "set":
				cache[req.key] = req.value
				req.response <- contextResponse{
					value: req.value,
					found: true,
					error: nil,
				}

			case "clear":
				cache = make(map[string]int)
				req.response <- contextResponse{
					value: 0,
					found: true,
					error: nil,
				}

			default:
				req.response <- contextResponse{
					value: 0,
					found: false,
					error: fmt.Errorf("unknown operation: %s", req.operation),
				}
			}
			close(req.response)

		case <-s.shutdown:
			// Drain any remaining requests
		drainLoop:
			for {
				select {
				case req := <-s.requests:
					req.response <- contextResponse{
						value: 0,
						found: false,
						error: fmt.Errorf("service shutting down"),
					}
					close(req.response)
				default:
					break drainLoop
				}
			}
			return
		}
	}
}

// Get retrieves a value from the cache
func (s *ModelContextService) Get(key string) (int, bool) {
	responseChan := make(chan contextResponse, 1)
	request := contextRequest{
		operation: "get",
		key:       key,
		value:     0,
		response:  responseChan,
	}

	select {
	case s.requests <- request:
		select {
		case response := <-responseChan:
			return response.value, response.found
		case <-s.shutdown:
			return 0, false
		}
	case <-s.shutdown:
		return 0, false
	}
}

// Set stores a value in the cache
func (s *ModelContextService) Set(key string, value int) error {
	responseChan := make(chan contextResponse, 1)
	request := contextRequest{
		operation: "set",
		key:       key,
		value:     value,
		response:  responseChan,
	}

	select {
	case s.requests <- request:
		select {
		case response := <-responseChan:
			return response.error
		case <-s.shutdown:
			return fmt.Errorf("service shutting down")
		}
	case <-s.shutdown:
		return fmt.Errorf("service shutting down")
	}
}

// Clear removes all entries from the cache
func (s *ModelContextService) Clear() error {
	responseChan := make(chan contextResponse, 1)
	request := contextRequest{
		operation: "clear",
		key:       "",
		value:     0,
		response:  responseChan,
	}

	select {
	case s.requests <- request:
		response := <-responseChan
		return response.error
	case <-s.shutdown:
		return fmt.Errorf("service shutting down")
	}
}

// Shutdown gracefully stops the service
func (s *ModelContextService) Shutdown(ctx context.Context) error {
	close(s.shutdown)

	// Wait for worker to finish or context timeout
	select {
	case <-s.done:
		s.wg.Wait()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

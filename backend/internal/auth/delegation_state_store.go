package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrDelegationStateNotFound = errors.New("delegation state not found")

type DelegationStateRecord struct {
	UserID         int64  `json:"userId"`
	ResourceServer string `json:"resourceServer"`
	CodeVerifier   string `json:"codeVerifier"`
	SessionHash    string `json:"sessionHash"`
}

type DelegationStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewDelegationStateStore(client *redis.Client, ttl time.Duration) *DelegationStateStore {
	return &DelegationStateStore{
		client: client,
		prefix: "delegation-state:",
		ttl:    ttl,
	}
}

func (s *DelegationStateStore) Create(ctx context.Context, userID int64, resourceServer, sessionHash string) (string, DelegationStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	record := DelegationStateRecord{
		UserID:         userID,
		ResourceServer: resourceServer,
		CodeVerifier:   codeVerifier,
		SessionHash:    sessionHash,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", DelegationStateRecord{}, err
	}

	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", DelegationStateRecord{}, fmt.Errorf("save delegation state: %w", err)
	}

	return state, record, nil
}

func (s *DelegationStateStore) Consume(ctx context.Context, state string) (DelegationStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return DelegationStateRecord{}, ErrDelegationStateNotFound
	}
	if err != nil {
		return DelegationStateRecord{}, fmt.Errorf("consume delegation state: %w", err)
	}

	var record DelegationStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return DelegationStateRecord{}, fmt.Errorf("decode delegation state: %w", err)
	}

	return record, nil
}

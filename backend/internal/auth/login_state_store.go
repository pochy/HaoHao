package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrLoginStateNotFound = errors.New("login state not found")

type LoginStateRecord struct {
	CodeVerifier string `json:"codeVerifier"`
	Nonce        string `json:"nonce"`
	ReturnTo     string `json:"returnTo"`
}

type LoginStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewLoginStateStore(client *redis.Client, ttl time.Duration) *LoginStateStore {
	return &LoginStateStore{
		client: client,
		prefix: "oidc-state:",
		ttl:    ttl,
	}
}

func (s *LoginStateStore) Create(ctx context.Context, returnTo string) (string, LoginStateRecord, error) {
	state, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	codeVerifier, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	nonce, err := randomToken(32)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	record := LoginStateRecord{
		CodeVerifier: codeVerifier,
		Nonce:        nonce,
		ReturnTo:     returnTo,
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return "", LoginStateRecord{}, err
	}

	if err := s.client.Set(ctx, s.prefix+state, payload, s.ttl).Err(); err != nil {
		return "", LoginStateRecord{}, fmt.Errorf("save login state: %w", err)
	}

	return state, record, nil
}

func (s *LoginStateStore) Consume(ctx context.Context, state string) (LoginStateRecord, error) {
	raw, err := s.client.GetDel(ctx, s.prefix+state).Bytes()
	if errors.Is(err, redis.Nil) {
		return LoginStateRecord{}, ErrLoginStateNotFound
	}
	if err != nil {
		return LoginStateRecord{}, fmt.Errorf("consume login state: %w", err)
	}

	var record LoginStateRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return LoginStateRecord{}, fmt.Errorf("decode login state: %w", err)
	}

	return record, nil
}

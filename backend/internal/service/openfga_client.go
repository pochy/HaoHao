package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	openfga "github.com/openfga/go-sdk"
	fgaclient "github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
)

type OpenFGAClientConfig struct {
	APIURL               string
	StoreID              string
	AuthorizationModelID string
	APIToken             string
	Timeout              time.Duration
}

type OpenFGACondition struct {
	Name    string
	Context map[string]any
}

type OpenFGATuple struct {
	User      string
	Relation  string
	Object    string
	Condition *OpenFGACondition
}

type OpenFGAClient interface {
	Check(ctx context.Context, tuple OpenFGATuple, context map[string]any) (bool, error)
	BatchCheck(ctx context.Context, tuples []OpenFGATuple, context map[string]any) ([]bool, error)
	ListObjects(ctx context.Context, user string, relation string, objectType string, context map[string]any) ([]string, error)
	WriteTuples(ctx context.Context, tuples []OpenFGATuple) error
	DeleteTuples(ctx context.Context, tuples []OpenFGATuple) error
}

type OpenFGASDKClient struct {
	client  fgaclient.SdkClient
	modelID string
	timeout time.Duration
}

func NewOpenFGASDKClient(cfg OpenFGAClientConfig) (*OpenFGASDKClient, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	storeID := strings.TrimSpace(cfg.StoreID)
	modelID := strings.TrimSpace(cfg.AuthorizationModelID)
	if apiURL == "" || storeID == "" || modelID == "" {
		return nil, fmt.Errorf("openfga api url, store id, and authorization model id are required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	clientCfg := &fgaclient.ClientConfiguration{
		ApiUrl:               apiURL,
		StoreId:              storeID,
		AuthorizationModelId: modelID,
		HTTPClient:           &http.Client{Timeout: timeout},
	}
	if token := strings.TrimSpace(cfg.APIToken); token != "" {
		clientCfg.Credentials = &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: token,
			},
		}
	}

	client, err := fgaclient.NewSdkClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("create openfga sdk client: %w", err)
	}

	return &OpenFGASDKClient{
		client:  client,
		modelID: modelID,
		timeout: timeout,
	}, nil
}

func (c *OpenFGASDKClient) Check(ctx context.Context, tuple OpenFGATuple, contextMap map[string]any) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("openfga client is not configured")
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()

	body := fgaclient.ClientCheckRequest{
		User:     tuple.User,
		Relation: tuple.Relation,
		Object:   tuple.Object,
	}
	if len(contextMap) > 0 {
		body.Context = &contextMap
	}
	options := fgaclient.ClientCheckOptions{
		AuthorizationModelId: openfga.ToPtr(c.modelID),
	}
	resp, err := c.client.Check(ctx).Body(body).Options(options).Execute()
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	return resp.GetAllowed(), nil
}

func (c *OpenFGASDKClient) BatchCheck(ctx context.Context, tuples []OpenFGATuple, contextMap map[string]any) ([]bool, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("openfga client is not configured")
	}
	if len(tuples) == 0 {
		return nil, nil
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()

	checks := make([]fgaclient.ClientBatchCheckItem, 0, len(tuples))
	for i, tuple := range tuples {
		item := fgaclient.ClientBatchCheckItem{
			User:          tuple.User,
			Relation:      tuple.Relation,
			Object:        tuple.Object,
			CorrelationId: fmt.Sprintf("%d", i),
		}
		if len(contextMap) > 0 {
			item.Context = &contextMap
		}
		checks = append(checks, item)
	}
	options := fgaclient.BatchCheckOptions{
		AuthorizationModelId: openfga.ToPtr(c.modelID),
	}
	resp, err := c.client.BatchCheck(ctx).Body(fgaclient.ClientBatchCheckRequest{Checks: checks}).Options(options).Execute()
	if err != nil {
		return nil, err
	}
	result := make([]bool, len(tuples))
	if resp == nil || resp.Result == nil {
		return result, nil
	}
	for i := range tuples {
		item, ok := (*resp.Result)[fmt.Sprintf("%d", i)]
		if !ok || item.Error != nil {
			continue
		}
		result[i] = item.GetAllowed()
	}
	return result, nil
}

func (c *OpenFGASDKClient) ListObjects(ctx context.Context, user string, relation string, objectType string, contextMap map[string]any) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("openfga client is not configured")
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()

	body := fgaclient.ClientListObjectsRequest{
		User:     user,
		Relation: relation,
		Type:     objectType,
	}
	if len(contextMap) > 0 {
		body.Context = &contextMap
	}
	options := fgaclient.ClientListObjectsOptions{
		AuthorizationModelId: openfga.ToPtr(c.modelID),
	}
	resp, err := c.client.ListObjects(ctx).Body(body).Options(options).Execute()
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.GetObjects(), nil
}

func (c *OpenFGASDKClient) WriteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("openfga client is not configured")
	}
	if len(tuples) == 0 {
		return nil
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()

	body := make(fgaclient.ClientWriteTuplesBody, 0, len(tuples))
	for _, tuple := range tuples {
		body = append(body, openFGATupleToSDK(tuple))
	}
	options := fgaclient.ClientWriteOptions{
		AuthorizationModelId: openfga.ToPtr(c.modelID),
	}
	_, err := c.client.WriteTuples(ctx).Body(body).Options(options).Execute()
	return err
}

func (c *OpenFGASDKClient) DeleteTuples(ctx context.Context, tuples []OpenFGATuple) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("openfga client is not configured")
	}
	if len(tuples) == 0 {
		return nil
	}
	ctx, cancel := c.callContext(ctx)
	defer cancel()

	body := make(fgaclient.ClientDeleteTuplesBody, 0, len(tuples))
	for _, tuple := range tuples {
		body = append(body, openfga.TupleKeyWithoutCondition{
			User:     tuple.User,
			Relation: tuple.Relation,
			Object:   tuple.Object,
		})
	}
	options := fgaclient.ClientWriteOptions{
		AuthorizationModelId: openfga.ToPtr(c.modelID),
	}
	_, err := c.client.DeleteTuples(ctx).Body(body).Options(options).Execute()
	return err
}

func (c *OpenFGASDKClient) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}

func openFGATupleToSDK(tuple OpenFGATuple) openfga.TupleKey {
	item := openfga.TupleKey{
		User:     tuple.User,
		Relation: tuple.Relation,
		Object:   tuple.Object,
	}
	if tuple.Condition != nil {
		conditionContext := tuple.Condition.Context
		if conditionContext == nil {
			conditionContext = map[string]any{}
		}
		item.Condition = &openfga.RelationshipCondition{
			Name:    tuple.Condition.Name,
			Context: &conditionContext,
		}
	}
	return item
}

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	oac *clientWithResponses
}

func NewClient(url string, opts ...ClientOption) (*Client, error) {
	oac, err := newClientWithResponses(url, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		oac: oac,
	}, nil
}

func (c *Client) CreateNamespace(ctx context.Context, req CreateNamespaceRequest, reqEditors ...RequestEditorFn) (Namespace, error) {
	res, err := c.oac.CreateNamespaceWithResponse(ctx, req, reqEditors...)
	if err != nil {
		return Namespace{}, err
	}
	if res.StatusCode() == http.StatusCreated {
		return res.JSON201.Data, nil
	}
	return Namespace{}, getResErr(res)
}

func (c *Client) ListNamespaces(ctx context.Context, params *ListNamespacesParams, reqEditors ...RequestEditorFn) ([]Namespace, error) {
	res, err := c.oac.ListNamespacesWithResponse(ctx, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) UpdateNamespace(ctx context.Context, namespaceID string, req UpdateNamespaceRequest, reqEditors ...RequestEditorFn) (Namespace, error) {
	res, err := c.oac.UpdateNamespaceWithResponse(ctx, namespaceID, req, reqEditors...)
	if err != nil {
		return Namespace{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return Namespace{}, getResErr(res)
}

func (c *Client) DeleteNamespace(ctx context.Context, namespaceID string, reqEditors ...RequestEditorFn) error {
	res, err := c.oac.DeleteNamespaceWithResponse(ctx, namespaceID, reqEditors...)
	if err != nil {
		return err
	}
	if res.StatusCode() == http.StatusNoContent {
		return nil
	}
	return getResErr(res)
}

func (c *Client) CreateOperator(ctx context.Context, namespaceID string, req CreateOperatorRequest, reqEditors ...RequestEditorFn) (Operator, error) {
	res, err := c.oac.CreateOperatorWithResponse(ctx, namespaceID, req, reqEditors...)
	if err != nil {
		return Operator{}, err
	}
	if res.JSON201 != nil {
		return res.JSON201.Data, nil
	}
	return Operator{}, getResErr(res)
}

func (c *Client) GetOperator(ctx context.Context, namespaceID, operatorID string, reqEditors ...RequestEditorFn) (Operator, error) {
	res, err := c.oac.GetOperatorWithResponse(ctx, namespaceID, operatorID, reqEditors...)
	if err != nil {
		return Operator{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return Operator{}, getResErr(res)
}

func (c *Client) GetOperatorStats(ctx context.Context, namespaceID, operatorID string, reqEditors ...RequestEditorFn) (ServerStatsMsg, error) {
	res, err := c.oac.GetOperatorStatsWithResponse(ctx, namespaceID, operatorID, reqEditors...)
	if err != nil {
		return ServerStatsMsg{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return ServerStatsMsg{}, getResErr(res)
}

func (c *Client) ListOperators(ctx context.Context, namespaceID string, params *ListOperatorsParams, reqEditors ...RequestEditorFn) ([]Operator, error) {
	res, err := c.oac.ListOperatorsWithResponse(ctx, namespaceID, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) UpdateOperator(ctx context.Context, namespaceID, operatorID string, req UpdateOperatorRequest, reqEditors ...RequestEditorFn) (Operator, error) {
	res, err := c.oac.UpdateOperatorWithResponse(ctx, namespaceID, operatorID, UpdateOperatorJSONRequestBody{Data: &req}, reqEditors...)
	if err != nil {
		return Operator{}, err
	}
	if res.JSON200 != nil {
		return *res.JSON200, nil
	}
	return Operator{}, getResErr(res)
}

func (c *Client) DeleteOperator(ctx context.Context, namespaceID, operatorID string, unmanageAccounts bool, reqEditors ...RequestEditorFn) error {
	res, err := c.oac.DeleteOperatorWithResponse(ctx, namespaceID, operatorID, &DeleteOperatorParams{UnmanageAccounts: &unmanageAccounts}, reqEditors...)
	if err != nil {
		return err
	}
	if res.StatusCode() == http.StatusNoContent {
		return nil
	}
	return getResErr(res)
}

func (c *Client) GenerateOperatorProxyToken(ctx context.Context, namespaceID, operatorID string, reqEditors ...RequestEditorFn) (string, error) {
	res, err := c.oac.GenerateOperatorProxyTokenWithResponse(ctx, namespaceID, operatorID, reqEditors...)
	if err != nil {
		return "", err
	}
	if res.JSON201 != nil {
		return res.JSON201.Data.Token, nil
	}
	return "", getResErr(res)
}

func (c *Client) GetOperatorNATSConfig(ctx context.Context, namespaceID, operatorID string, reqEditors ...RequestEditorFn) ([]byte, error) {
	res, err := c.oac.GetOperatorNATSConfigWithResponse(ctx, namespaceID, operatorID, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() == http.StatusOK {
		return res.Body, nil
	}
	return nil, getResErr(res)
}

func (c *Client) GetOperatorProxyConnectionStatus(ctx context.Context, namespaceID, operatorID string, reqEditors ...RequestEditorFn) (OperatorNATSStatus, error) {
	res, err := c.oac.GetOperatorProxyConnectionStatusWithResponse(ctx, namespaceID, operatorID, reqEditors...)
	if err != nil {
		return OperatorNATSStatus{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return OperatorNATSStatus{}, getResErr(res)
}

func (c *Client) CreateAccount(ctx context.Context, namespaceID, operatorID string, req CreateAccountRequest, reqEditors ...RequestEditorFn) (Account, error) {
	res, err := c.oac.CreateAccountWithResponse(ctx, namespaceID, operatorID, req, reqEditors...)
	if err != nil {
		return Account{}, err
	}
	if res.JSON201 != nil {
		return res.JSON201.Data, nil
	}
	return Account{}, getResErr(res)
}

func (c *Client) GetAccount(ctx context.Context, namespaceID, accountID string, reqEditors ...RequestEditorFn) (Account, error) {
	res, err := c.oac.GetAccountWithResponse(ctx, namespaceID, accountID, reqEditors...)
	if err != nil {
		return Account{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return Account{}, getResErr(res)
}

func (c *Client) ListAccounts(ctx context.Context, namespaceID, operatorID string, params *ListAccountsParams, reqEditors ...RequestEditorFn) ([]Account, error) {
	res, err := c.oac.ListAccountsWithResponse(ctx, namespaceID, operatorID, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) UpdateAccount(ctx context.Context, namespaceID, accountID string, req UpdateAccountRequest, reqEditors ...RequestEditorFn) (Account, error) {
	res, err := c.oac.UpdateAccountWithResponse(ctx, namespaceID, accountID, UpdateAccountJSONRequestBody{Data: &req}, reqEditors...)
	if err != nil {
		return Account{}, err
	}
	if res.JSON200 != nil {
		return *res.JSON200, nil
	}
	return Account{}, getResErr(res)
}

func (c *Client) DeleteAccount(ctx context.Context, namespaceID, accountID string, unmanage bool, reqEditors ...RequestEditorFn) error {
	res, err := c.oac.DeleteAccountWithResponse(ctx, namespaceID, accountID, &DeleteAccountParams{Unmanage: &unmanage}, reqEditors...)
	if err != nil {
		return err
	}
	if res.StatusCode() == http.StatusNoContent {
		return nil
	}
	return getResErr(res)
}

func (c *Client) CreateUser(ctx context.Context, namespaceID, accountID string, req CreateUserRequest, reqEditors ...RequestEditorFn) (User, error) {
	res, err := c.oac.CreateUserWithResponse(ctx, namespaceID, accountID, req, reqEditors...)
	if err != nil {
		return User{}, err
	}
	if res.JSON201 != nil {
		return res.JSON201.Data, nil
	}
	return User{}, getResErr(res)
}

func (c *Client) GetUser(ctx context.Context, namespaceID, userID string, reqEditors ...RequestEditorFn) (User, error) {
	res, err := c.oac.GetUserWithResponse(ctx, namespaceID, userID, reqEditors...)
	if err != nil {
		return User{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return User{}, getResErr(res)
}

func (c *Client) ListUsers(ctx context.Context, namespaceID, accountID string, params *ListUsersParams, reqEditors ...RequestEditorFn) ([]User, error) {
	res, err := c.oac.ListUsersWithResponse(ctx, namespaceID, accountID, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) UpdateUser(ctx context.Context, namespaceID, userID string, req UpdateUserRequest, reqEditors ...RequestEditorFn) (User, error) {
	res, err := c.oac.UpdateUserWithResponse(ctx, namespaceID, userID, req, reqEditors...)
	if err != nil {
		return User{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return User{}, getResErr(res)
}

func (c *Client) GetUserCreds(ctx context.Context, namespaceID, userID string, reqEditors ...RequestEditorFn) ([]byte, error) {
	res, err := c.oac.GetUserCredsWithResponse(ctx, namespaceID, userID, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.StatusCode() == http.StatusOK {
		return res.Body, nil
	}
	return nil, getResErr(res)
}

func (c *Client) ListUserJWTIssuances(ctx context.Context, namespaceID, userID string, reqEditors ...RequestEditorFn) ([]UserJWTIssuance, error) {
	res, err := c.oac.ListUserJWTIssuancesWithResponse(ctx, namespaceID, userID, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil && res.JSON200.Data != nil {
		return *res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) DeleteUser(ctx context.Context, namespaceID, userID string, reqEditors ...RequestEditorFn) error {
	res, err := c.oac.DeleteUserWithResponse(ctx, namespaceID, userID, reqEditors...)
	if err != nil {
		return err
	}
	if res.StatusCode() == http.StatusNoContent {
		return nil
	}
	return getResErr(res)
}

func (c *Client) GetStream(ctx context.Context, namespaceID, accountID string, streamName string, reqEditors ...RequestEditorFn) (Stream, error) {
	res, err := c.oac.GetStreamWithResponse(ctx, namespaceID, accountID, streamName, reqEditors...)
	if err != nil {
		return Stream{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return Stream{}, getResErr(res)
}

func (c *Client) ListStreams(ctx context.Context, namespaceID, accountID string, reqEditors ...RequestEditorFn) ([]Stream, error) {
	res, err := c.oac.ListStreamsWithResponse(ctx, namespaceID, accountID, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) FetchStreamMessages(ctx context.Context, namespaceID, accountID string, streamName string, params *FetchStreamMessagesParams, reqEditors ...RequestEditorFn) ([]StreamMessage, error) {
	res, err := c.oac.FetchStreamMessagesWithResponse(ctx, namespaceID, accountID, streamName, params, reqEditors...)
	if err != nil {
		return nil, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return nil, getResErr(res)
}

func (c *Client) GetStreamMessageContent(ctx context.Context, namespaceID, accountID string, streamName string, sequence uint64, reqEditors ...RequestEditorFn) (StreamMessageContent, error) {
	res, err := c.oac.GetStreamMessageContentWithResponse(ctx, namespaceID, accountID, streamName, sequence, reqEditors...)
	if err != nil {
		return StreamMessageContent{}, err
	}
	if res.JSON200 != nil {
		return res.JSON200.Data, nil
	}
	return StreamMessageContent{}, getResErr(res)
}

type response interface {
	GetBody() []byte
	StatusCode() int
}

func getResErr(res response) error {
	var rerr ResponseError
	body := res.GetBody()
	if len(body) > 0 {
		if err := json.Unmarshal(body, &rerr); err != nil {
			return err
		}
	}

	if rerr.Error.Message == "" {
		return fmt.Errorf("(%d) %s", res.StatusCode(), http.StatusText(res.StatusCode()))
	}

	errStr := rerr.Error.Message
	if len(rerr.Error.Details) > 0 {
		errStr += ": " + strings.Join(rerr.Error.Details, ", ")
	}

	return errors.New(errStr)
}

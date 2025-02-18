package irys

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/Ja7ad/irys/currency"
	"github.com/Ja7ad/irys/errors"
	"github.com/Ja7ad/irys/types"
	"github.com/Ja7ad/irys/utils/logger"
	"github.com/hashicorp/go-retryablehttp"
)

type Client struct {
	mu       *sync.Mutex
	client   *retryablehttp.Client
	network  Node
	currency currency.Currency
	contract string
	logging  logger.Logger
	debug    bool
}

type Irys interface {
	// GetPrice return fee base on fileSize in byte for selected currency
	GetPrice(ctx context.Context, fileSize int) (*big.Int, error)

	// BasicUpload file with calculate price and topUp balance base on price (this is slower for upload)
	BasicUpload(ctx context.Context, file []byte, tags ...types.Tag) (types.Transaction, error)
	// Upload file with check balance
	Upload(ctx context.Context, file []byte, tags ...types.Tag) (types.Transaction, error)
	// ChunkUpload upload file chunk concurrent for big files (min size: 500 KB, max size: 95 MB)
	//
	// chunkId used for resume upload, chunkId expired after 30 min.
	//
	// Note: this feature is experimental, maybe not work.
	ChunkUpload(ctx context.Context, file io.Reader, chunkId string, tags ...types.Tag) (types.Transaction, error)

	// Download get file with header details
	Download(ctx context.Context, txId string) (*types.File, error)
	// GetMetaData get transaction details
	GetMetaData(ctx context.Context, txId string) (types.Transaction, error)

	// GetBalance return current balance in irys node
	GetBalance(ctx context.Context) (*big.Int, error)
	// TopUpBalance top up your balance base on your amount in selected node
	TopUpBalance(ctx context.Context, amount *big.Int) error

	// GetReceipt get receipt information from node
	GetReceipt(ctx context.Context, txId string) (types.Receipt, error)

	// Close stop irys client request
	Close()
}

// New create IrysClient object
func New(node Node, currency currency.Currency, debug bool, options ...Option) (Irys, error) {
	irys := new(Client)

	httpClient := &http.Client{
		Timeout: 300 * time.Second,
	}

	irys.client = retryablehttp.NewClient()
	irys.client.HTTPClient = httpClient

	irys.network = node
	irys.currency = currency
	irys.mu = new(sync.Mutex)

	irys.debug = debug

	irys.client.RetryMax = 5
	irys.client.RetryWaitMin = 1 * time.Second
	irys.client.RetryWaitMax = 30 * time.Second
	irys.client.ErrorHandler = retryablehttp.PassthroughErrorHandler

	for _, opt := range options {
		opt(irys)
	}

	if irys.logging == nil {
		logging, err := logger.New(logger.CONSOLE_HANDLER, logger.Options{
			Development:  false,
			Debug:        false,
			EnableCaller: true,
			SkipCaller:   4,
		})
		if err != nil {
			return nil, err
		}

		irys.logging = logging
	}

	irys.client.Logger = irys.logging

	if !debug {
		irys.client.Logger = nil
	}

	if irys.client.HTTPClient.Transport == nil {
		irys.client.HTTPClient.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}

	irys.mu.Lock()
	contract, err := irys.getTokenContractAddress(node, currency)
	if err != nil {
		return nil, err
	}
	irys.mu.Unlock()

	irys.contract = contract

	return irys, nil
}

func (c *Client) Close() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := c.client.HTTPClient.Transport.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

func (c *Client) getTokenContractAddress(node Node, currency currency.Currency) (string, error) {
	r, err := c.client.Get(string(node))
	if err != nil {
		return "", err
	}

	if err := statusCheck(r); err != nil {
		return "", err
	}

	resp, err := decodeBody[types.NodeInfo](r.Body)
	if err != nil {
		return "", err
	}

	if v, ok := resp.Addresses[currency.GetName()]; ok {
		c.debugMsg("set currency address %s base on currency %s", v, currency.GetName())
		return v, nil
	}

	return "", errors.ErrCurrencyIsInvalid
}

func (c *Client) debugMsg(msg string, args ...any) {
	if c.debug {
		c.logging.Debug(fmt.Sprintf(msg, args...))
	}
}

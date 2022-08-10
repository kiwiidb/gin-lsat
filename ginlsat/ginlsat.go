package ginlsat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kiwiidb/gin-lsat/ln"
	"github.com/kiwiidb/gin-lsat/lsat"
	"github.com/kiwiidb/gin-lsat/macaroon"
	macaroonutils "github.com/kiwiidb/gin-lsat/macaroon"
	"github.com/kiwiidb/gin-lsat/utils"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
)

const (
	LND_CLIENT_TYPE   = "LND"
	LNURL_CLIENT_TYPE = "LNURL"
)

const (
	LSAT_TYPE_FREE = "FREE"
	LSAT_TYPE_PAID = "PAID"
	LSAT_HEADER    = "application/vnd.lsat.v1.full"
)

const (
	FREE_CONTENT_MESSAGE      = "Free Content"
	PROTECTED_CONTENT_MESSAGE = "Protected Content"
	PAYMENT_REQUIRED_MESSAGE  = "Payment Required"
)

type LsatInfo struct {
	Type     string
	Preimage lntypes.Preimage
	Mac      *macaroon.MacaroonIdentifier
	Amount   int64
	Error    error
}

type GinLsatMiddleware struct {
	AmountFunc func(req *http.Request) (amount int64)
	LNClient   ln.LNClient
}

func NewLsatMiddleware(lnClientConfig *ln.LNClientConfig,
	amountFunc func(req *http.Request) (amount int64)) (*GinLsatMiddleware, error) {
	lnClient, err := InitLnClient(lnClientConfig)
	if err != nil {
		return nil, err
	}
	middleware := &GinLsatMiddleware{
		AmountFunc: amountFunc,
		LNClient:   lnClient,
	}
	return middleware, nil
}

func InitLnClient(lnClientConfig *ln.LNClientConfig) (ln.LNClient, error) {
	var lnClient ln.LNClient
	err := godotenv.Load(".env")
	if err != nil {
		return lnClient, errors.New("Failed to load .env file")
	}

	switch lnClientConfig.LNClientType {
	case LND_CLIENT_TYPE:
		lnClient, err = ln.NewLNDclient(lnClientConfig.LNDConfig)
		if err != nil {
			return lnClient, fmt.Errorf("Error initializing LN client: %s", err.Error())
		}
	case LNURL_CLIENT_TYPE:
		lnClient, err = ln.NewLNURLClient(lnClientConfig.LNURLConfig)
		if err != nil {
			return lnClient, fmt.Errorf("Error initializing LN client: %s", err.Error())
		}
	default:
		return lnClient, fmt.Errorf("LN Client type not recognized: %s", lnClientConfig.LNClientType)
	}

	return lnClient, nil
}

func (lsatmiddleware *GinLsatMiddleware) Handler(c *gin.Context) {
	//First check for presence of authorization header
	authField := c.Request.Header.Get("Authorization")
	mac, preimage, err := utils.ParseLsatHeader(authField)
	if err != nil {
		// No Authorization present, check if client supports LSAT
		acceptLsatField := c.Request.Header.Get("Accept")
		if strings.Contains(acceptLsatField, "application/vnd.lsat.v1.full") {
			lsatmiddleware.SetLSATHeader(c)
			return
		}
		// Set LSAT type Free if client does not support LSAT
		c.Set("LSAT", &LsatInfo{
			Type: LSAT_TYPE_FREE,
		})
		return
	}
	//LSAT Header is present, verify it
	err = lsat.VerifyLSAT(mac, utils.GetRootKey(), preimage)
	if err != nil {
		//not a valid LSAT
		c.Error(err)
		c.Set("LSAT", &LsatInfo{
			Error: err,
		})
		return
	}
	//LSAT verification ok, mark client as having paid
	c.Set("LSAT", &LsatInfo{
		Type: LSAT_TYPE_PAID,
	})

}

func (lsatmiddleware *GinLsatMiddleware) SetLSATHeader(c *gin.Context) {
	// Generate invoice and token
	ctx := context.Background()
	lnInvoice := lnrpc.Invoice{
		Value: lsatmiddleware.AmountFunc(c.Request),
		Memo:  "LSAT",
	}
	LNClientConn := &ln.LNClientConn{
		LNClient: lsatmiddleware.LNClient,
	}
	invoice, paymentHash, err := LNClientConn.GenerateInvoice(ctx, lnInvoice, c.Request)
	if err != nil {
		c.Error(err)
		c.Set("LSAT", &LsatInfo{
			Error: err,
		})
		return
	}
	macaroonString, err := macaroonutils.GetMacaroonAsString(paymentHash)
	if err != nil {
		c.Error(err)
		c.Set("LSAT", &LsatInfo{
			Error: err,
		})
		return
	}
	c.Writer.Header().Set("WWW-Authenticate", fmt.Sprintf("LSAT macaroon=%s, invoice=%s", macaroonString, invoice))
	c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
		"code":    http.StatusPaymentRequired,
		"message": PAYMENT_REQUIRED_MESSAGE,
	})
}

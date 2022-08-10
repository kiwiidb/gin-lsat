package macaroon

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"

	"github.com/kiwiidb/gin-lsat/utils"
	"github.com/lightningnetwork/lnd/lntypes"
	"gopkg.in/macaroon.v2"
)

type MacaroonIdentifier struct {
	Version     uint16
	PaymentHash lntypes.Hash
	TokenId     [32]byte
}

func GetMacaroonAsString(paymentHash lntypes.Hash) (string, error) {
	// rootKey, err := generateRootKey()
	// if err != nil {
	// 	return "", err
	// }
	rootKey := utils.GetRootKey()

	identifier, err := generateMacaroonIdentifier(paymentHash)
	if err != nil {
		return "", err
	}

	mac, err := macaroon.New(
		rootKey[:],
		identifier,
		"LSAT",
		macaroon.LatestVersion,
	)
	if err != nil {
		return "", err
	}

	macBytes, err := mac.MarshalBinary()
	if err != nil {
		return "", err
	}

	macaroonString := base64.StdEncoding.EncodeToString(macBytes)
	return macaroonString, err
}

func generateMacaroonIdentifier(paymentHash lntypes.Hash) ([]byte, error) {
	tokenId, err := generateTokenId()
	if err != nil {
		return nil, err
	}

	id := &MacaroonIdentifier{
		Version:     0,
		PaymentHash: paymentHash,
		TokenId:     tokenId,
	}

	var identifier bytes.Buffer
	enc := gob.NewEncoder(&identifier)
	if err := enc.Encode(id); err != nil {
		return nil, err
	}
	return identifier.Bytes(), err
}

func generateTokenId() ([32]byte, error) {
	var tokenId [32]byte
	_, err := rand.Read(tokenId[:])
	return tokenId, err
}

func generateRootKey() ([32]byte, error) {
	var rootKey [32]byte
	_, err := rand.Read(rootKey[:])
	return rootKey, err
}

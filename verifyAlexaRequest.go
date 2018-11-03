/*
Most of this file comes from https://github.com/mikeflynn/go-alexa. Amazon makes us
verify that requests to our webservice come from them only, so we must check their certs.
*/
package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/appengine/urlfetch"

	"google.golang.org/appengine/log"
)

func HTTPError(ctx context.Context, w http.ResponseWriter, logMsg string, err string, errCode int) {
	if logMsg != "" {
		log.Debugf(ctx, logMsg)
	}

	http.Error(w, err, errCode)
}

// Run all mandatory Amazon security checks on the request.
func validateRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Check for debug bypass flag
	devFlag := r.URL.Query().Get("_dev")

	isDev := devFlag != ""

	if !isDev {
		isRequestValid := IsValidAlexaRequest(ctx, w, r)
		if !isRequestValid {
			return
		}
	}
}

// IsValidAlexaRequest handles all the necessary steps to validate that an incoming http.Request has actually come from
// the Alexa service. If an error occurs during the validation process, an http.Error will be written to the provided http.ResponseWriter.
// The required steps for request validation can be found on this page:
// https://developer.amazon.com/public/solutions/alexa/alexa-skills-kit/docs/developing-an-alexa-skill-as-a-web-service#hosting-a-custom-skill-as-a-web-service
func IsValidAlexaRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	certURL := r.Header.Get("SignatureCertChainUrl")

	// Verify certificate URL
	if !verifyCertURL(certURL) {

		HTTPError(ctx, w, "Invalid cert URL: "+certURL, "Not Authorized", 401)
		return false
	}

	// Fetch certificate data
	certContents, err := readCert(ctx, certURL)
	if err != nil {
		HTTPError(ctx, w, err.Error(), "Not Authorized", 401)
		return false
	}

	// Decode certificate data
	block, _ := pem.Decode(certContents)
	if block == nil {
		HTTPError(ctx, w, "Failed to parse certificate PEM.", "Not Authorized", 401)
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		HTTPError(ctx, w, err.Error(), "Not Authorized", 401)
		return false
	}

	// Check the certificate date
	if time.Now().Unix() < cert.NotBefore.Unix() || time.Now().Unix() > cert.NotAfter.Unix() {
		HTTPError(ctx, w, "Amazon certificate expired.", "Not Authorized", 401)
		return false
	}

	// Check the certificate alternate names
	foundName := false
	for _, altName := range cert.Subject.Names {
		if altName.Value == "echo-api.amazon.com" {
			foundName = true
		}
	}

	if !foundName {
		HTTPError(ctx, w, "Amazon certificate invalid.", "Not Authorized", 401)
		return false
	}

	// Verify the key
	publicKey := cert.PublicKey
	encryptedSig, _ := base64.StdEncoding.DecodeString(r.Header.Get("Signature"))

	// Make the request body SHA1 and verify the request with the public key
	var bodyBuf bytes.Buffer
	hash := sha1.New()
	_, err = io.Copy(hash, io.TeeReader(r.Body, &bodyBuf))
	if err != nil {
		HTTPError(ctx, w, err.Error(), "Internal Error", 500)
		return false
	}
	r.Body = ioutil.NopCloser(&bodyBuf)

	err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA1, hash.Sum(nil), encryptedSig)
	if err != nil {
		HTTPError(ctx, w, "Signature match failed.", "Not Authorized", 401)
		return false
	}

	return true
}

func readCert(ctx context.Context, certURL string) ([]byte, error) {
	client := urlfetch.Client(ctx)
	request, err := http.NewRequest("GET", certURL, nil)

	if err != nil {
		return nil, fmt.Errorf("could not create GET request for certURL: %s", certURL)
	}

	cert, err := client.Do(request)

	if err != nil {
		return nil, errors.New("could not download Amazon cert file")
	}
	defer cert.Body.Close()
	certContents, err := ioutil.ReadAll(cert.Body)
	if err != nil {
		return nil, errors.New("could not read Amazon cert file")
	}

	return certContents, nil
}

func verifyCertURL(path string) bool {
	link, _ := url.Parse(path)

	if link.Scheme != "https" {
		return false
	}

	if link.Host != "s3.amazonaws.com" && link.Host != "s3.amazonaws.com:443" {
		return false
	}

	if !strings.HasPrefix(link.Path, "/echo.api/") {
		return false
	}

	return true
}

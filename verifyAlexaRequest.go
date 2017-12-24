/*
Most of this file comes from https://github.com/mikeflynn/go-alexa. Amazon makes us
verify that requests to our webservice come from them only.
*/
package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func HTTPError(w http.ResponseWriter, logMsg string, err string, errCode int) {
	if logMsg != "" {
		log.Println(logMsg)
	}

	http.Error(w, err, errCode)
}

// Run all mandatory Amazon security checks on the request.
func validateRequest(w http.ResponseWriter, r *http.Request) {
	// Check for debug bypass flag
	devFlag := r.URL.Query().Get("_dev")

	isDev := devFlag != ""

	if !isDev {
		isRequestValid := IsValidAlexaRequest(w, r)
		if !isRequestValid {
			return
		}
	}
}

// IsValidAlexaRequest handles all the necessary steps to validate that an incoming http.Request has actually come from
// the Alexa service. If an error occurs during the validation process, an http.Error will be written to the provided http.ResponseWriter.
// The required steps for request validation can be found on this page:
// https://developer.amazon.com/public/solutions/alexa/alexa-skills-kit/docs/developing-an-alexa-skill-as-a-web-service#hosting-a-custom-skill-as-a-web-service
func IsValidAlexaRequest(w http.ResponseWriter, r *http.Request) bool {
	certURL := r.Header.Get("SignatureCertChainUrl")

	// Verify certificate URL
	if !verifyCertURL(certURL) {

		HTTPError(w, "Invalid cert URL: "+certURL, "Not Authorized", 401)
		return false
	}

	// Fetch certificate data
	certContents, err := readCert(certURL)
	if err != nil {
		HTTPError(w, err.Error(), "Not Authorized", 401)
		return false
	}

	// Decode certificate data
	block, _ := pem.Decode(certContents)
	if block == nil {
		HTTPError(w, "Failed to parse certificate PEM.", "Not Authorized", 401)
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		HTTPError(w, err.Error(), "Not Authorized", 401)
		return false
	}

	// Check the certificate date
	if time.Now().Unix() < cert.NotBefore.Unix() || time.Now().Unix() > cert.NotAfter.Unix() {
		HTTPError(w, "Amazon certificate expired.", "Not Authorized", 401)
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
		HTTPError(w, "Amazon certificate invalid.", "Not Authorized", 401)
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
		HTTPError(w, err.Error(), "Internal Error", 500)
		return false
	}
	//log.Println(bodyBuf.String())
	r.Body = ioutil.NopCloser(&bodyBuf)

	err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA1, hash.Sum(nil), encryptedSig)
	if err != nil {
		HTTPError(w, "Signature match failed.", "Not Authorized", 401)
		return false
	}

	return true
}

func readCert(certURL string) ([]byte, error) {
	cert, err := http.Get(certURL)
	if err != nil {
		return nil, errors.New("Could not download Amazon cert file.")
	}
	defer cert.Body.Close()
	certContents, err := ioutil.ReadAll(cert.Body)
	if err != nil {
		return nil, errors.New("Could not read Amazon cert file.")
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

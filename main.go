package main

import (
	"golang.org/x/oauth2"
)

config := &oauth2.Config{
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
    Endpoint:     oauth2.Endpoint{
        TokenURL: "https://api.paycor.com/v1/token",
    },
}

token, err := config.PasswordCredentialsToken(context.Background(), "username", "password")
if err != nil {
    log.Fatal(err)
}
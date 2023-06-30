package slavart

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"slavartdl/lib/config"
	"slavartdl/lib/helpers"
)

const (
	Api = "https://api.divolt.xyz"

	RequestChannel = "01G9AZ9AMWDV227YA7FQ5RV8WB"
	UploadChannel  = "01G9AZ9Q2R5VEGVPQ4H99C01YP"

	SlavartBotId = "01G9824MQPGD7GVYR0F6A6GJ2Q"
)

// interact with the slavart divolt server (its a self-hosted instance of revolt) and use its api

// this only has what I need
type RevoltMessage struct {
	Mentions []string `json:"mentions"`
	Embeds   []struct {
		Description string `json:"description"`
	} `json:"embeds"`
}

// check if bot is down
func GetSlavartBotOnlineStatus(sessionToken string) (bool, error) {
	serverMemberResponse := struct {
		Online bool `json:"online"`
	}{}

	err := helpers.JsonApiRequest(
		http.MethodGet,
		Api+"/users/"+SlavartBotId,
		&serverMemberResponse,
		map[string]string{},
		map[string]string{
			"X-Session-Token": sessionToken,
		},
	)

	return serverMemberResponse.Online, err
}

// this is because we dont want to spam the bot service with a single account, so we use multiple
func GetRandomDivoltSessionToken() (string, error) {
	rand.Seed(time.Now().UnixNano())

	sessionTokens := config.Public.DivoltSessionTokens
	length := len(sessionTokens)
	if length == 0 {
		return "", errors.New("no session tokens found")
	} else if length == 1 {
		return sessionTokens[0], nil
	}
	random := rand.Intn(length - 1)

	return sessionTokens[random], nil
}

// send message in request channel
func SendDownloadMessage(sessionToken, link string) (string, error) {
	downloadRequestResponse := struct {
		MessageId string `json:"_id"`
	}{}

	err := helpers.JsonApiRequest(
		http.MethodPost,
		Api+"/channels/"+RequestChannel+"/messages",
		&downloadRequestResponse,
		map[string]string{
			"content": "!dl " + link,
		},
		map[string]string{
			"X-Session-Token": sessionToken,
		},
	)

	return downloadRequestResponse.MessageId, err
}

// get last 100 messages in upload channel
func GetUploadMessages(sessionToken string) ([]RevoltMessage, error) {
	downloadRequestFinishedTestResponse := []RevoltMessage{}

	err := helpers.JsonApiRequest(
		http.MethodGet,
		Api+"/channels/"+UploadChannel+"/messages",
		&downloadRequestFinishedTestResponse,
		map[string]string{
			"sort":          "Latest",
			"include_users": "false",
		},
		map[string]string{
			"X-Session-Token": sessionToken,
		},
	)

	return downloadRequestFinishedTestResponse, err
}

// see if we can find an upload message for the link we want (even if its for another user)
func SearchForDownloadLinkInUploadMessages(link string, messages []RevoltMessage) (string, bool) {
	for _, revoltMessage := range messages {
		regex := regexp.MustCompile(`(?m)Your requested link\, (.*)\, is now available for download:\n \*\*Download Link\*\*\n (.*)`)
		matches := regex.FindAllStringSubmatch(revoltMessage.Embeds[0].Description, -1)[0]

		if matches[1] == link {
			return matches[2], true
		}
	}

	return "", false
}

func GetDownloadLinkFromSlavart(link string, quality int, timeoutTime time.Time) (string, error) {
	sessionToken, err := GetRandomDivoltSessionToken()
	if err != nil {
		return "", err
	}

	realLink := link
	if quality != 0 {
		realLink = fmt.Sprintf("%s %d", link, quality-1)
	}

	messages, err := GetUploadMessages(sessionToken)
	if err != nil {
		return "", err
	}

	if downloadLink, ok := SearchForDownloadLinkInUploadMessages(realLink, messages); ok {
		return downloadLink, nil
	}

	online, err := GetSlavartBotOnlineStatus(sessionToken)
	if err != nil {
		return "", err
	}

	if !online {
		return "", errors.New("bot isn't online")
	}

	if _, err := SendDownloadMessage(sessionToken, realLink); err != nil {
		return "", err
	}

	for {
		if timeoutTime.Before(time.Now()) {
			return "", errors.New("timed-out before download link could be found")
		}

		time.Sleep(time.Second * 10)

		messages, err := GetUploadMessages(sessionToken)
		if err != nil {
			return "", err
		}

		if downloadLink, ok := SearchForDownloadLinkInUploadMessages(realLink, messages); ok {
			return downloadLink, nil
		}
	}
}

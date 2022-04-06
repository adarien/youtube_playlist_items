package main

import (
	"encoding/json"
	"fmt"
	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
)

// getToken uses a Context and Config to retrieve a Token.
// It returns the retrieved Token and any error encountered.
func getToken(config *oauth2.Config) (*oauth2.Token, error) {
	cacheFile, err := getPathTokenCacheFile()
	if err != nil {
		err := fmt.Errorf("unable to get path to cached credential file: %s", err)
		return nil, err
	}

	token, err := getTokenFromFile(cacheFile)
	if err != nil {
		token, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		err = saveToken(cacheFile, token)
		if err != nil {
			return nil, err
		}
	}

	return token, nil
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token and any error encountered.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	instruction := "Go to the following link in your browser then type the authorization code"
	fmt.Printf("%s: \n%v\n", instruction, authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		err = fmt.Errorf("unable to read authorization code: %s", err)
		return nil, err
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		err = fmt.Errorf("unable to retrieve token from web: %s", err)
		return nil, err
	}

	return token, nil
}

// getPathTokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename and any error encountered.
func getPathTokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	err = os.MkdirAll(tokenCacheDir, 0700)
	if err != nil {
		return "", err
	}
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("youtube-go-quickstart.json")), err
}

// getTokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func getTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeError := f.Close()
		if err == nil {
			err = closeError
		}
	}()
	if err != nil {
		err = fmt.Errorf("unable to close file: %s", err)
		return nil, err
	}

	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	return t, err
}

// saveToken uses a file path to create a file and store the token in it.
// It returns any error encountered.
func saveToken(file string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		err = fmt.Errorf("unable to cache oauth token: %s", err)
		return err
	}

	defer func() {
		closeError := f.Close()
		if err == nil {
			err = closeError
		}
	}()
	if err != nil {
		err = fmt.Errorf("unable to close file: %s", err)
		return err
	}

	err = json.NewEncoder(f).Encode(token)
	if err != nil {
		return err
	}

	return nil
}

// getPlaylistsInfo get playlists information
// It returns PlaylistListResponse struct and any error encountered.
func getPlaylistsInfo(service *youtube.Service, channelId string) (*youtube.PlaylistListResponse, error) {
	part := []string{"snippet", "contentDetails"}
	call := service.Playlists.List(part)
	if channelId != "" {
		call = call.ChannelId(channelId)
	}
	call = call.MaxResults(25)

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("getPlaylistsInfo not call: %v", err)
	}

	return response, nil
}

func getChannelsLists(service *youtube.Service, part []string, username string) (*youtube.ChannelListResponse, error) {
	call := service.Channels.List(part)
	call = call.ForUsername(username)

	response, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("channel not call: %v", err)
	}
	if len(response.Items) == 0 {
		return nil, fmt.Errorf("incorrect userName")
	}

	return response, nil
}

func getListsID(service *youtube.Service, response *youtube.ChannelListResponse) ([]string, error) {
	channelID := response.Items[0].Id
	response2, err := getPlaylistsInfo(service, channelID)
	if err != nil {
		return nil, err
	}

	var playlists []string
	for _, playlist := range response2.Items {
		if playlist.Snippet.Title != "Favorites" {
			playlists = append(playlists, playlist.Id)
		}
	}

	return playlists, nil
}

func getItemInfo(playlistResponse *youtube.PlaylistItemListResponse) {
	// TODO: create struct
	for _, playlistItem := range playlistResponse.Items {
		title := playlistItem.Snippet.Title
		videoId := playlistItem.Snippet.ResourceId.VideoId
		position := playlistItem.Snippet.Position
		publishedAt := playlistItem.Snippet.PublishedAt
		videoOwnerChannelTitle := playlistItem.Snippet.VideoOwnerChannelTitle
		videoOwnerChannelId := playlistItem.Snippet.VideoOwnerChannelId
		placeInList := playlistItem.Snippet.Position

		// TODO: record to DB
		fmt.Printf("%4d :: %11v :: %v :: %v :: %v :: %v :: %v\r\n", placeInList,
			videoId, title, position, publishedAt, videoOwnerChannelTitle, videoOwnerChannelId)
	}
}

func getListItems(service *youtube.Service, playlistsID []string) error {
	nextPageToken := ""
	for _, ID := range playlistsID {
		fmt.Println(ID)
		for {
			playlistCall := service.PlaylistItems.List([]string{"snippet"}).
				PlaylistId(ID).
				MaxResults(50).
				PageToken(nextPageToken)

			playlistResponse, err := playlistCall.Do()
			if err != nil {
				return fmt.Errorf("error fetching playlist items: %s", err)
			}

			getItemInfo(playlistResponse)

			nextPageToken = playlistResponse.NextPageToken
			if nextPageToken == "" {
				break
			}
		}
		fmt.Println()
	}

	return nil
}

// initLogger initializes the logger
func initLogger() *logrus.Logger {
	var logger = logrus.New()

	Formatter := new(logrus.TextFormatter)
	Formatter.TimestampFormat = "2006-01-02 15:04:05"
	Formatter.FullTimestamp = true
	Formatter.ForceColors = true
	Formatter.PadLevelText = true
	logger.SetFormatter(Formatter)
	logger.SetOutput(colorable.NewColorableStdout())

	return logger
}

func initCredential(CredentialFilePath string) ([]byte, error) {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	cs, err := ioutil.ReadFile(CredentialFilePath)
	if err != nil {
		return nil, err
	}

	return cs, err
}

func Run() error {
	ctx := context.Background()
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf(" - - - unable to read .env file - - - : %s", err)
	}
	CredentialFilePath := viper.GetString("CREDENTIAL_FILEPATH")
	userName := viper.GetString("USERNAME")

	cs, err := initCredential(CredentialFilePath)
	if err != nil {
		return fmt.Errorf(" - - - unable to read client secret file - - - : %s", err)
	}

	config, err := google.ConfigFromJSON(cs, youtube.YoutubeReadonlyScope)
	if err != nil {
		return fmt.Errorf(" - - - unable to parse client secret file to config - - - : %s", err)
	}

	token, err := getToken(config)
	if err != nil {
		return fmt.Errorf(" - - - unable to get token - - - : %s", err)
	}

	service, err := youtube.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
	if err != nil {
		return fmt.Errorf(" - - - unable to create client - - - : %s", err)
	}

	part := []string{"snippet", "contentDetails"}
	resp, err := getChannelsLists(service, part, userName)
	if err != nil {
		return fmt.Errorf(" - - - unable to get channel list - - - : %s", err)
	}

	IDs, err := getListsID(service, resp)
	if err != nil {
		return fmt.Errorf(" - - - unable to get playlists ID - - - : %s", err)
	}

	err = getListItems(service, IDs)
	if err != nil {
		return fmt.Errorf(" - - - unable to get playlists Items - - - : %s", err)
	}

	return nil
}

func main() {
	log := initLogger()

	err := Run()
	if err != nil {
		log.Fatal(err)
	}
}

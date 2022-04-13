package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
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
	"time"
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

type playlistMeta struct {
	ID    string
	Title string
	Count int64
}

func getListsID(service *youtube.Service, response *youtube.ChannelListResponse) ([]playlistMeta, error) {
	channelID := response.Items[0].Id
	response2, err := getPlaylistsInfo(service, channelID)
	if err != nil {
		return nil, err
	}

	var playlists []playlistMeta
	for _, playlist := range response2.Items {
		if playlist.Snippet.Title != "Favorites" {
			meta := playlistMeta{}
			meta.ID = playlist.Id
			meta.Title = playlist.Snippet.Title
			meta.Count = playlist.ContentDetails.ItemCount
			playlists = append(playlists, meta)
		}
	}

	return playlists, nil
}

type TrackInfo struct {
	PlaylistTitle          string    `json:"playlistName,omitempty"`
	VideoID                string    `json:"videoId,omitempty"`
	TrackTitle             string    `json:"title,omitempty"`
	PublishedAt            string    `json:"publishedAt,omitempty"`
	VideoOwnerChannelTitle string    `json:"videoOwnerChannelTitle,omitempty"`
	VideoOwnerChannelId    string    `json:"videoOwnerChannelId,omitempty"`
	PlaylistID             string    `json:"playlistid,omitempty"`
	Position               int64     `json:"position,omitempty"`
	Created                time.Time `json:"created,omitempty"`
}

func getItemInfo(conn *ServiceSQL, playlistResponse *youtube.PlaylistItemListResponse, meta playlistMeta) {
	for _, playlistItem := range playlistResponse.Items {
		oi := TrackInfo{}
		oi.PlaylistTitle = meta.Title
		oi.VideoID = playlistItem.Snippet.ResourceId.VideoId
		oi.TrackTitle = playlistItem.Snippet.Title
		oi.PublishedAt = playlistItem.Snippet.PublishedAt
		oi.VideoOwnerChannelTitle = playlistItem.Snippet.VideoOwnerChannelTitle
		oi.VideoOwnerChannelId = playlistItem.Snippet.VideoOwnerChannelId
		oi.PlaylistID = meta.ID
		oi.Position = playlistItem.Snippet.Position

		// fmt.Println(oi)
		// fmt.Println(oi.VideoID)

		// record to DB
		err := conn.PostProduct(oi)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func getListItems(service *youtube.Service, playlistMeta []playlistMeta) error {
	nextPageToken := ""
	conn := New()

	for _, meta := range playlistMeta {
		fmt.Print(meta.Title, " ", meta.Count)
		for {
			playlistCall := service.PlaylistItems.List([]string{"snippet"}).
				PlaylistId(meta.ID).
				MaxResults(50).
				PageToken(nextPageToken)

			playlistResponse, err := playlistCall.Do()
			if err != nil {
				return fmt.Errorf("error fetching playlist items: %s", err)
			}

			getItemInfo(conn, playlistResponse, meta)

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

	meta, err := getListsID(service, resp)
	if err != nil {
		return fmt.Errorf(" - - - unable to get playlists ID - - - : %s", err)
	}

	err = getListItems(service, meta)
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

type DB struct {
	DB *sql.DB
}

type ServiceSQL struct {
	db *DB
}

func New() *ServiceSQL {
	dbClient := Connect()
	return &ServiceSQL{db: dbClient}
}

func Connect() *DB {
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatal(err)
	}

	driverName := viper.GetString("DRIVER")
	host := viper.GetString("HOST")
	port := viper.GetString("PORT")
	userName := viper.GetString("USER")
	dbname := viper.GetString("DBNAME")
	sslMode := viper.GetString("SSLMODE")
	password := viper.GetString("PASSWORD")

	dataSourceName := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s password=%s",
		host, port, userName, dbname, sslMode, password)
	// fmt.Println(dataSourceName)
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		logrus.Fatal(err)
	}

	return &DB{DB: db}
}

func (db *DB) InsertProductDB(ti TrackInfo) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := "insert into playlists_info (playlisttitle, position, videoid, tracktitle, publishedat, playlistid, videoownerchannelid, videoownerchanneltitle) values ($1, $2, $3, $4, $5, $6, $7, $8)"
	_, err = tx.Exec(query, ti.PlaylistTitle, ti.Position, ti.VideoID, ti.TrackTitle, ti.PublishedAt, ti.PlaylistID, ti.VideoOwnerChannelId, ti.VideoOwnerChannelTitle)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *ServiceSQL) PostProduct(ti TrackInfo) error {
	err := s.db.InsertProductDB(ti)
	if err != nil {
		return fmt.Errorf(" - - - unable to write to DB - - - : %s", err)
	}
	return nil
}

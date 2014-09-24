package main

import "net/http"
import "log"
import "fmt"
import "encoding/json"
import "io/ioutil"
import "strconv"
import "sort"

import "github.com/mitchellh/mapstructure"

const API_KEY = "51356937edaa4eeef5a3f6ba7e52f0d7"

type SongData struct {
	Name      string `mapstructure:"title" json:"title"`
	Artist    string `mapstructure:"artist" json:"artist"`
	Url       string `mapstructure:"stream_url_raw" json:"url"`
	DateLoved int    `mapstructure:"dateloved" json:"dateLoved"`
}

type ByDateLoved []SongData

func (a ByDateLoved) Len() int           { return len(a) }
func (a ByDateLoved) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDateLoved) Less(i, j int) bool { return a[i].DateLoved > a[j].DateLoved }

func main() {
	http.HandleFunc("/api/grab", getSongs)
	http.Handle("/", http.FileServer(http.Dir('.')))
	http.ListenAndServe(":8080", nil)
}

func getSongs(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query()["username"][0]
	log.Println("username is " + username)

	data, err := query(username)
	if err != nil {
		log.Println("abort!")

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sort.Sort(ByDateLoved(data))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string][]SongData{"results": data})
}

func query(username string) ([]SongData, error) {
	var burstSize = 20

	songs := make(chan []SongData, burstSize)
	errs := make(chan error, burstSize)

	var currentIndex = 1
	var songData []SongData

requestLoop:
	for {
		select {
		case err := <-errs:
			log.Printf("Found error on channel: %s", err)
			close(songs)
			break requestLoop
		default:
			for i := 0; i < burstSize; i++ {
				index := currentIndex + i
				go func(index int) {
					newSongs, err := getSong(username, index)

					if err != nil {
						log.Printf("Err: %s", err)
						errs <- err
						songs <- []SongData{}
						return
					}

					songs <- newSongs
				}(index)
			}
			currentIndex += burstSize
		}

		for idx := 0; idx < burstSize; idx++ {
			songItem, ok := <-songs

			if ok != true {
				log.Fatal("RUHROH - that shouldn't happen!")
				continue
			}

			songData = append(songData, songItem...)
		}

		log.Printf("Finished burst %d: %+v", currentIndex, len(songData))
	}

	log.Print("Finished loading all songs")
	return songData, nil
}

func getSong(username string, index int) ([]SongData, error) {
	log.Printf("About to query %s [%d]", username, index)
	url := fmt.Sprintf("http://api.hypem.com/playlist/loved/%s/json/%d/data.js?key=%s", username, index, API_KEY)
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Failed to create request...")
		return []SongData{}, err
	}

	req.Header.Add("User-Agent", "http://hypeliberator.com v1.1")
	req.Header.Add("Host", "api.hypem.com")
	req.Header.Add("Proxy-Connection", "close")
	req.Header.Add("Connection", "close")

	resp, err := client.Do(req)

	if err != nil {
		log.Println("Failed to make request...")
		return []SongData{}, err
	}

	if resp.StatusCode != 200 {
		log.Printf("Got response code %d", resp.StatusCode)
		return []SongData{}, fmt.Errorf("Got response code %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	var tmp interface{}
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &tmp)

	if err != nil {
		log.Println("FAILED TO PARSE JSON")
		return []SongData{}, err
	}

	// tmp is the giant JSON blob
	var songs []SongData
	for key, value := range tmp.(map[string]interface{}) {
		_, err := strconv.ParseInt(key, 10, 32)

		// Otherwise it's not a number and probably not a song
		if err != nil {
			// log.Printf("Failed to parse %s", key)
			continue
		}

		var song SongData
		// value
		err = mapstructure.Decode(value, &song)
		if err != nil {
			log.Println("Failed to decode! %v", value)
			continue
		}

		songs = append(songs, song)
	}

	return songs, nil
}

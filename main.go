package main

import "net/http"
import "log"
import "fmt"
import "encoding/json"
import "io/ioutil"
import "strconv"
import "sort"

import "github.com/mitchellh/mapstructure"

// API_KEY is the key used to authenticate to the Hype Machine web service
// Extracted from the UberHype APK
const API_KEY = "51356937edaa4eeef5a3f6ba7e52f0d7"

// REQ_BURST_SIZE is the number of concurrent requests
// created at the end of each run - where a run is a
// burst of requests.
const REQ_BURST_SIZE = 5

// SongData represents a bridge structure to extract the data
// from the Hype Machine service pass it onto the Hype Liberator
// web interface
type SongData struct {
	Name      string `mapstructure:"title" json:"title"`
	Artist    string `mapstructure:"artist" json:"artist"`
	Url       string `mapstructure:"stream_url_raw" json:"url"`
	DateLoved int    `mapstructure:"dateloved" json:"dateLoved"`
}

// ByDateLoved is a super simple interface to get date sorting working
// I'm not entirely sure why it was necessary to make an interface just
// for that. But y'know.
type ByDateLoved []SongData

func (a ByDateLoved) Len() int           { return len(a) }
func (a ByDateLoved) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByDateLoved) Less(i, j int) bool { return a[i].DateLoved > a[j].DateLoved }

// Main starts the web service and listens in on localhost:4567 by default
// TODO(nickhs): Add in command line argument parsing to change these defaults
func main() {
	serverURL := "0.0.0.0:4567"

	// map the api call to getSongs
	http.HandleFunc("/api/grab", getSongs)

	// everything else is just attempted as a static
	// file serve. NB this should be handled by nginx
	// or apache in production and is mainly here for
	// development.
	http.Handle("/", http.FileServer(http.Dir('.')))
	log.Println("Starting...", serverURL)
	err := http.ListenAndServe(serverURL, nil)
	if err != nil {
		log.Fatal("Failed to start!", err)
	}
}

// Entry point for the scraping service, called from /api/grab
// Requires a valid username to be passed in
func getSongs(w http.ResponseWriter, r *http.Request) {
	// Username is passed in as a query string parameter
	username := r.URL.Query().Get("username")
	if len(username) == 0 {
		http.Error(w, "Need to specify a username as a query parameter", http.StatusBadRequest)
		return
	}

	log.Println("username is " + username)

	// kick's off the scraping service
	data, err := query(username)
	if err != nil {
		log.Println("Failed to scrape for " + username)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// sort the songs returned by date loved
	sort.Sort(ByDateLoved(data))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// return the songs like as a key, value such as:
	// {"results": [{SongData}...{SongData}]}
	// to prevent jSON top level array attacks
	json.NewEncoder(w).Encode(map[string][]SongData{"results": data})
}

// Query is the meat of the concurrent scraping logic.
// it kicks off bursts of requests according to REQ_BURST_SIZE above.
// It also manages the responses and determines when we've found all
// the songs for the user/traversed all the pages.
//
// Because Hype Machine's API can be best described as special
// this process is a little convulated.
func query(username string) ([]SongData, error) {
	// channel to pass list of songs onto
	songs := make(chan []SongData, REQ_BURST_SIZE)

	// channel to pass errors on. Errors include 5xx, and 4xx
	// errors, where 404 notably indicates the
	// end of the paginated pages (i.e. break
	// the request loop).
	errs := make(chan error, REQ_BURST_SIZE)

	// what paginated page we're looking at
	var currentIndex = 1

	// all the songs returned so far
	var songData []SongData

requestLoop:
	for { // while True loop - contains scraping and draining logic
		select {
		// select ensures that if we have anything on errs
		// we go down that code path
		case err := <-errs:
			log.Printf("Found error on channel: %s", err)
			// no more song fetching
			close(songs)
			// we're done - let's get out of the requestLoop
			break requestLoop
		default:
			// go for another burst
			for i := 0; i < REQ_BURST_SIZE; i++ {
				index := currentIndex + i
				// put the scraper on a new go coroutine
				go func(index int) {
					newSongs, err := getSong(username, index)

					// if we got an error put that on errs
					if err != nil {
						log.Printf("Err: %s", err)
						errs <- err
						// we need to add a blank array here
						// as we cannot close the songs channel
						// immediately as other requests with a
						// lower pagination index may still
						// be in flight.
						songs <- []SongData{}
						return
					}

					songs <- newSongs
				}(index)
			}

			currentIndex += REQ_BURST_SIZE
		}

		// We have know filled our channels with songs we need
		// to drain them.
		for idx := 0; idx < REQ_BURST_SIZE; idx++ {
			// block here until something appears on the
			// songs channel
			songItem, ok := <-songs

			if ok != true {
				// if this happens we probably have bigger problems.
				log.Fatal("Failed to get songData[] from channel?!")
			}

			// flatten arrays and individual songData[]
			// to the general songData[] slice.
			songData = append(songData, songItem...)
		}

		log.Printf("Finished burst %d: %+v", currentIndex, len(songData))
	}

	// we get here after we break out of the request loop
	return songData, nil
}

// getSong is the actual extraction logic.
// Fetches and transforms a JSON response
// from Hype Machine and get back a SongData[] slice.
func getSong(username string, index int) ([]SongData, error) {
	url := fmt.Sprintf("http://api.hypem.com/playlist/loved/%s/json/%d/data.js?key=%s", username, index, API_KEY)
	log.Printf("About to query %s [%d] - %s", username, index, url)
	client := &http.Client{}

	// Logic to build the request
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

	// Request complete - process response
	var tmp interface{}
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &tmp)

	if err != nil {
		log.Println("Failed to parse JSON")
		return []SongData{}, err
	}

	// tmp is the giant JSON blob
	// we can't do this directly since
	// HypeM's responses are once again special
	// and instead of returning an array of Song objects
	// they return an object with the keys being 1, 2 .. 20.
	// Special. I know.
	//
	// Instead we cast this to a map[string]interface{}
	// and then attempt type conversions to make it work
	var songs []SongData
	for key, value := range tmp.(map[string]interface{}) {
		_, err := strconv.ParseInt(key, 10, 32)

		// Otherwise it's not a number and probably not a song
		if err != nil {
			// log.Printf("Failed to parse %s", key)
			continue
		}

		// convert a map into a SongData struct as defined above
		var song SongData
		err = mapstructure.Decode(value, &song)
		if err != nil {
			log.Println("Failed to decode! %v", value)
			continue
		}

		songs = append(songs, song)
	}

	return songs, nil
}

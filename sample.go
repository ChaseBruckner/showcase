// The entry point of this code sample is the Changes function. This function recieves a hash-map of recreation.gov API request
// that will return campsite availability data for a specified campground. The elements are keyed by their appropriate
// req_id (campground_id & yyyy-mm). Each element in this hash-map is refered to as an active request. Please refer to the
// ActiveRequests struct below for the object schema.

package availability

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type RespCampsiteData struct {
	Campsites map[string]CampsiteMetadata `json:"campsites"`
}

type CampsiteMetadata struct {
	Availabilities map[string]string `json:"availabilities"`
	CampsiteId     string            `json:"campsite_id"`
}

type ActiveRequests struct {
	ReqId        string          `firestore:"req_id" json:"req_id"`
	CampgroundId string          `firestore:"campground_id" json:"campground_id"`
	Url          string          `firestore:"url" json:"url"`
	Update       bool            `firestore:"update" json:"update"`
	ActiveDays   map[string]bool `firestore:"active_days" json:"active_days"`
}

var WEBSITE string = "https://www.recreation.gov/camping/campgrounds/"

func makeRequest(reqURL string, refURL string) (io.Reader, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authority", "www.recreation.gov")
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("cache-control", "no-cache, no-store, must-revalidate")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("referer", refURL)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-gpc", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.67 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return strings.NewReader(string(body)), nil
}

func Changes(reqData map[string]ActiveRequests) map[string]map[string]map[string]bool {

	var changes = make(map[string]map[string]map[string]bool)
	// schema:
	// Any change in availability that occurs will be documented with this schema for future uses in updating personalized subscriber information
	// and constructing alerts to subscribers when needed
	// Only dates that experience changes will be added to their respective campground hash-map along with the date's specific campsite availability
	// 	{
	// 		"campground_id_yyyy-mm" : {
	// 			"yyyy-mm-dd": {
	// 				"campsite_id 1": bool,
	// 				"campsite_id 2": bool,
	// 				"campsite_id 3": bool,
	// 				etc....
	// 			},
	// 			"yyyy-mm-dd": {..}
	// 		},
	// 		"campground_id_yyyy-mm" : {...}
	// 	}

	// makes request for new availability data for each active request in reqData and compares it to the old availibility data to create list of changes
	for reqId, reqInfo := range reqData {

		// constructs hash-map for old availability and new availability data
		var oldAvlbty = make(map[string]map[string]bool)
		var newAvlbty = make(map[string]map[string]bool)
		// schema:
		// the bool indicates whether the campsite is av
		// 		"yyyy-mm-dd": {
		// 			"campsite_id 1": bool,
		// 			"campsite_id 2": bool,
		// 			"campsite_id 3": bool,
		// 			etc....
		// 		},
		// 		"date(yyyy-mm-dd)": {...}

		// constructs GCP Cloud Storage file name
		file := fmt.Sprintf("%s_availability.json", reqId)

		// reads in old availability info from CloudStorage into memory. Similar function code can be found on GCP Cloud Storage documentation
		avlbtyByte, err := downloadFileIntoMemory(file)
		if err != nil {
			log.Printf("unable to load %s into memory from cloud storage due to: %s", file, err)
			reqInfo.Update = false
			continue
		}
		// if file does not exist, avlbtyByte will be of length 0 indicating the campground has not been previously monitored and new file will be
		// constructed via the temp file that will be created
		if len(avlbtyByte) > 0 {
			if err := json.Unmarshal(avlbtyByte, &oldAvlbty); err != nil {
				log.Printf("unable to unmarshall %s due to: %s", file, err)
				reqInfo.Update = false
				continue
			}
		}

		// constructs file name for a JSON file that will temorarily hold the new availability data
		tempFile := fmt.Sprintf("temp_%s_availability.json", reqId)
		// This temp file serves 2 self-healing puposes:
		// 	1) Before the old availability data is updated with the new data, all database changes to subscriber information and nessecary alerts need
		// 	   to successfully complete. If an error occurs in any of these procedures and the old availability data is updated, the
		// 	   subsequent monitoring scans will not re-trigger any availability changes that previously experienced procedural errors, Meaning subscriber
		// 	   data will fall out of sync with availability information. This metric is tracked via the recInfo.Update attribute. Any request whose
		// 	   recinfo.Update is false will not have its availability data updated to ensure all neccesary database changese and alerts take place on the next
		// 	   error free monitoring scan. This process enables the system to be self-healing.
		// 	2) If the campground has not been previously monitored, the temp file will be added to Cloud Storage to act as the baseline/old availability for
		// 	   sebsequent monitoring scans once renamed. Upon completion of the program procedures (assuming procedrures were completed error free), the temp
		//     file will be renamed to the old availability file name that will be accessed in future monitoring scans

		refURL := WEBSITE + reqInfo.CampgroundId

		// makes recration.gov GET request for new availability
		log.Printf("%s request start\n", reqId)
		resp_body, err := makeRequest(reqInfo.Url, refURL)
		if err != nil {
			log.Printf("unable to make request for %s availability data due to::  %s\n", reqId, err)
			reqInfo.Update = false
			continue
		}

		log.Printf("%s request complete\n", reqId)

		// decodes GET request response
		var req_resp RespCampsiteData
		// request response schema:
		// 	{
		// 		"campsite_id 1": {
		// 			"date(yyyy-mm-dd)": "Available",
		// 			"date(yyyy-mm-dd)": "Not Available",
		// 			etc....
		// 		},
		// 		"campsite_id 2": {...}
		// 	}
		decoder := json.NewDecoder(resp_body)
		if err := decoder.Decode(&req_resp); err != nil {
			log.Printf("unable to decode %s new availability data due to: %s\n", reqId, err)
			reqInfo.Update = false
			continue
		}

		// constructs new availability data by iterating through the GET request response and comparing to old availability data
		for campsite, campInfo := range req_resp.Campsites {
			for date, status := range campInfo.Availabilities {
				campDate := strings.Split(date, "T")[0]
				if _, present := newAvlbty[campDate]; !present {
					newAvlbty[campDate] = make(map[string]bool)
				}
				newAvlbty[campDate][campsite] = false
				if status == "Available" {
					newAvlbty[campDate][campsite] = true
				}
			}
		}

		// compares old availability data to new availability data to construct a list of changes betwteen the two data sets
		for date, campsites := range newAvlbty {

			// On rare occasions, recreation.gov will return a date that was not prevously included in past scans. If this occurs, a
			// change event is triggered and the new date will be added to the changes list.
			if _, present := oldAvlbty[date]; !present {

				if _, present := changes[reqId]; !present {
					changes[reqId] = make(map[string]map[string]bool)
					changes[reqId][date] = campsites
				}

				continue
			}

			for campsite, available := range campsites {
				// If a new campsite is added to the list of researvable campsites returned by recreation.gov, it will trigger change
				// event and the new date will be added to the changes list.
				if _, present := oldAvlbty[date][campsite]; !present {

					if _, present := changes[reqId]; !present {
						changes[reqId] = make(map[string]map[string]bool)
						changes[reqId][date] = campsites
					}
					break
				}

				// Records changes in campsite availability: If new availability is different from the old availability, a change event
				// will trigger and the new date will be added to the changes list along with breaking out of the campsite check loop.
				// This loop can be broken out of as no further change investigation needs to be completed because the new data for the
				// particular day has already been added to the changes list
				if available != oldAvlbty[date][campsite] {

					if _, present := changes[reqId]; !present {
						changes[reqId] = make(map[string]map[string]bool)
						changes[reqId][date] = campsites
					}
					break
				}
			}
		}

		// On occasion, recreation.gov will remove a campsite from the returned list of available campsites, posing the need to itterate through
		// the old availability to check for missing campsites in new availability. If this scenario occurs, a change event will trigger and the
		// new date will be added to the changes list.
		for date, campsites := range oldAvlbty {
			for campsite := range campsites {

				if _, present := newAvlbty[date][campsite]; !present {
					if _, present := changes[reqId]; !present {
						changes[reqId] = make(map[string]map[string]bool)
						changes[reqId][date] = campsites
					}
					break
				}
			}
		}

		// Writes updated availability data(tempFile) from memory to Cloud Stoorage denoted as a temporary file in order to not overwrite any
		// untracked changes/updates/notifactions. Similar function code can be found on GCP Cloud Storage documentation.
		// The temporary file name will later be updated/renamed upon successful error free completion of monitoring procedures.
		if err := uploadFileFromMemory(tempFile, newAvlbty); err != nil {
			log.Printf("unable to upload store %s new availability data due to: %s\n", reqId, err)
			reqInfo.Update = false
			continue
		}
	}

	return changes
}

// The addTripProcedures function showcased in this snippet represents a function called by multiple POST and PUT
// request handlers and ensures that database updates are atomic

package trip

type TripInfo struct {
	TripName     string              `firestore:"trip_name" json:"trip_name"`
	TripId       string              `firestore:"trip_id" json:"trip_id"`
	SubscriberId string              `firestore:"subscriber_id" json:"subscriber_id"`
	Location     string              `firestore:"location" json:"location"`
	Active       bool                `firestore:"active" json:"active"`
	Notification bool                `firestore:"notification" json:"notification"`
	Requests     map[string]string   `firestore:"requests" json:"requests"`
	Dates        map[string]DateInfo `firestore:"dates" json:"dates"`
}

type DateInfo struct {
	Available      bool              `firestore:"available" json:"available"`
	AvailabilityId string            `firestore:"availability_id" json:"availability_id"`
	Campsites      map[string]string `firestore:"campsites" json:"campsites"`
}

func addTripProcedures(procedures []string, trip TripInfo) ([]string, map[string]DateInfo, map[string]string, error) {
	// The procedures parameters is a list describing which Firestor collections require updates

	// The following variables are used to track which procedures have been completed as the function runs and information
	// pertaining the database updates completed thus far

	var proceduresCompleted []string
	var changedDates = make(map[string]DateInfo)
	var changedReqs = make(map[string]string)

	for _, procedure := range procedures {
		// For each procedure use the following series of if and else blocks to determine which type of database update to complete
		// Record each procedure as its completed

		// add trip data from tripMtd
		if procedure == "tripMtd" {
			if err := AddTripMtd(trip); err != nil {
				// If an error occurs during any of the updates return a list of the completed procedures.
				// This list will be used to undo the completed procedures ensuring that the database is not
				// left in a corrupted state. This function ensures atomic updates to multiple collection in
				// the database at the same time.

				return proceduresCompleted, changedDates, changedReqs, err
			}
			proceduresCompleted = append(proceduresCompleted, "tripMtd")
		} else if procedure == "subscriberMtd" {
			// add trip data from subscriber data
			if err := AddTripSubscriberMtd(trip.SubscriberId, trip.TripId); err != nil {
				return proceduresCompleted, changedDates, changedReqs, err
			}
			proceduresCompleted = append(proceduresCompleted, "subscriberMtd")
		} else if procedure == "subscriptionMtd" {
			// add subscription updates that have already been made
			for date, dateInfo := range trip.Dates {
				if err := AddSubscriptionMtd(trip.TripId, date, trip.Location); err != nil {
					proceduresCompleted = append(proceduresCompleted, "subscriptionMtd")
					return proceduresCompleted, changedDates, changedReqs, err
				}
				changedDates[date] = dateInfo
			}
			proceduresCompleted = append(proceduresCompleted, "subscriptionMtd")
		} else if procedure == "activeReqs" {
			// adds request to list of active requests if it is not already in the database
			for reqId := range trip.Requests {
				req, err := GetActiveReq(reqId)
				if err == ErrNoDoc {

					if err := AddActiveReq(reqId, trip.Dates); err != nil {
						proceduresCompleted = append(proceduresCompleted, "activeReqs")
						return proceduresCompleted, changedDates, changedReqs, err
					}

					return proceduresCompleted, changedDates, changedReqs, nil

				} else if err != nil {
					return proceduresCompleted, changedDates, changedReqs, err
				}
			}
			proceduresCompleted = append(proceduresCompleted, "activeReqs")
		}
	}
	// If no error occurs in the above code then return nil for the error value indicating that all procedures completed successfully
	return proceduresCompleted, changedDates, changedReqs, nil
}

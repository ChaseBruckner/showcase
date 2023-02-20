# Lantern Showcase

This sample code is a snippet from my Lantern project. Lantern is an outdoor recreation monitoring and alerting service. 

Each year recreation.gov releases reservation slots and permits for many recreation sites across the United States allowing a controlled amount of visitors to experience the beauty of our nearby nature. For the most popular recreation sites, these reservations get snatched up quickly! However, many reservations get canceled at the last minute. Lanter allows anyone to monitor these recreation sites for last minute availability. For instance, someone can provide interest in a specific campground and range of dates that they would like to visit said campground. They can also share preferences for their desired trip, such as: campsite type, presence of campsite attributes (e.g. electricity or water hookup), campsite size, or even specific campsites. Lantern will monitor the campgrounds availability and send them a notification via email when their desired trip and preferences are available. 

The code included in this sample is a snippet from the monitoring portion of the application. Its responsibility is to retrieve new availability data from the recreation.gov API, compare the data to old availability data, and construct a list of changes that will later be used to update subscriber information and send notifications when necessary.

I have also developed a REST API for Lantern that is ready to be hooked up with a frontend.

### Coming Soon to a DNS Server near you:

Check out [lantern.watch](https://lantern.watch/) for your camping adventures!

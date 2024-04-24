from DFReader import DFReader_binary
from DFReader import DFReader_text
from datetime import datetime
from timezonefinder import TimezoneFinder
import pytz
from kml_creator import KMLCreator
from kmz_writer import KMZWriter
import os

class DataExtractor:
    def __init__(self, filename):
        self.filename = filename

    def extract_data(self):
        # Check if the file exists
        if not os.path.isfile(self.filename):
            print(f"File {self.filename} does not exist.")
            return None

        # Open the binary file
        dfreader = DFReader_binary(self.filename)

        if self.filename.endswith('.log'):
            dfreader = DFReader_text(self.filename)
        else:
            dfreader = DFReader_binary(self.filename)

        start_timestamp = dfreader.clock.timebase
        end_timestamp = dfreader.clock.timestamp
        # Convert the timestamp to a datetime object
        start_datetime = datetime.utcfromtimestamp(start_timestamp)

        tf = TimezoneFinder()

        # Get the timezone
        if "GPS" not in dfreader.messages:
            print("no GPS data")
            return
        
        timezone_str = tf.certain_timezone_at(lat=dfreader.messages["GPS"].Lat, lng=dfreader.messages["GPS"].Lng)
        if timezone_str is None:
            print("Could not determine the timezone")
        else:
            # Create a timezone object
            timezone = pytz.timezone(timezone_str)

            # Localize the datetime object
            localized_datetime = pytz.utc.localize(start_datetime).astimezone(timezone)

            # Format the localized datetime object as a string
            local_date_str = localized_datetime.strftime("%m/%d/%Y")
            local_time_str = localized_datetime.strftime("%H:%M:%S")

            date = f"Local Date: {local_date_str}, Local Time: {local_time_str}"


        # Get the first message
        dfreader._parse_next()
        msg = dfreader.messages
        count = 0
        data = []  # Accumulator for GPS data

        # Get the values of the attributes
        fieldnames = msg['GPS']._fieldnames

        # Create a string from the fieldnames list
        fieldnames_str = ','.join(fieldnames)
            
        # Create a set to store seen times
        seen_times = set()

        # Iterate over all messages
        while msg is not None:
            if 'GPS' in msg:
                # Get all GPS values
                gps_values = msg['GPS']

                if gps_values.Lat != 0:
                    # Get the values of the fields
                    values = [getattr(gps_values, field, None) for field in fieldnames]

                    # Create a dictionary from fieldnames and values
                    entry_dict = dict(zip(fieldnames, values))

                    # Check if this time has been seen before
                    time = entry_dict.get('TimeMS') or entry_dict.get('TimeUS')
                    if time not in seen_times:
                        # Add this time to the set of seen times
                        seen_times.add(time)
                        data.append(entry_dict)      
                        count += 1

            # Get the next message
            dfreader._parse_next()
            print(f"{dfreader.percent:.1f}%")
            print(f"{count} unique records")
            if dfreader.percent > 99.99:
                break

            msg = dfreader.messages

        if len(data) == 0:
            print("No GPS Data in File")
            return
        
        return (data, date, fieldnames_str, start_timestamp, end_timestamp)
       
        
        

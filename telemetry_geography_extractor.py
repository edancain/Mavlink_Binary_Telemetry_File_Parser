import os
import csv
from DFReader import DFReader_binary
from DFReader import DFReader_text

class DataExtractor:
    def __init__(self, filename):
        self.filename = filename

    def __extract_data(self):
        # Check if the file exists
        if not os.path.isfile(self.filename):
            print(f"File {self.filename} does not exist.")
            return None

        # Open the binary file
        if self.filename.endswith('.log'):
            dfreader = DFReader_text(self.filename)
        else:
            dfreader = DFReader_binary(self.filename)

        # Get the timezone
        if "GPS" not in dfreader.messages:
            print("no GPS data")
            return None

        # Get the first message
        dfreader._parse_next()
        msg = dfreader.messages
        count = 0
        message_count = 0
        data = []  # Accumulator for GPS data

        # Get the values of the attributes
        fieldnames = msg['GPS']._fieldnames

        # Create a set to store seen times
        seen_times = set()

        # Iterate over all messages
        while msg is not None:
            message_count += 1
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

        print(f"Total messages: {message_count}")

        if len(data) == 0:
            print("No GPS Data in File")
            return None

        return (data )

    def write_gps_data_to_csv(self, output_file):
        # Extract GPS data
        extraction_result = self.__extract_data()
        if extraction_result is None:
            print("No GPS data to write to CSV.")
            return None

        gps_data = extraction_result

        # Define the fields to include in the CSV
        fieldnames = ['Lat', 'Lng', 'Alt']

        # Open the output CSV file in write mode
        with open(output_file, 'w', newline='') as csvfile:
            # Define the CSV writer
            writer = csv.DictWriter(csvfile, fieldnames=fieldnames)

            # Write the header row
            writer.writeheader()

            # Write GPS data to the CSV file
            for entry in gps_data:
                # Extract latitude, longitude, and altitude from the entry dictionary
                lat = entry.get('Lat')
                lng = entry.get('Lng')
                alt = entry.get('Alt')

                # Write a row to the CSV file with latitude, longitude, and altitude
                writer.writerow({'Lat': lat, 'Lng': lng, 'Alt': alt})

        print(f"GPS data written to: {output_file}")
        return output_file
       
        
        

import ast
import struct
#from pymavlink import DFReader
import pytz
import simplekml
import os
from datetime import datetime
from timezonefinder import TimezoneFinder

from DFReader import DFReader
from DFReader import DFReader_binary

def create_kml(data, filename, fieldnames, timestart, timeend):
    kml = simplekml.Kml()
    kml.name = filename
    kml.document.name = filename
    #kml.timespan.begin = timestart
    #kml.timespan.end = timeend
    # Create a list to store coordinates for line string
    line_coords = []

    for entry in data:
        # If entry is already a dictionary, use it directly
        if isinstance(entry, dict):
            entry_dict = entry
        else:
            # Create a dictionary from fieldnames and entry
            entry_dict = dict(zip(fieldnames, entry))

        # Add altitude to the coordinates
        coords_with_alt = (entry_dict['Lng'], entry_dict['Lat'], entry_dict['Alt'])

        # Add point with altitude
        pnt = kml.newpoint(name='',
                           coords=[coords_with_alt],
                           description='\n'.join(f'{key}: {value}' for key, value in entry_dict.items()))
        pnt.altitudemode = simplekml.AltitudeMode.absolute
        line_coords.append(coords_with_alt)

    for i in range(len(data) - 1):
        entry_dict1 = data[i]
        entry_dict2 = data[i+1]

        # Create the coordinates for the polygon
        coords = [(entry_dict1['Lng'], entry_dict1['Lat'], entry_dict1['Alt']), 
                  (entry_dict2['Lng'], entry_dict2['Lat'], entry_dict2['Alt']), 
                  (entry_dict2['Lng'], entry_dict2['Lat'], 0), 
                  (entry_dict1['Lng'], entry_dict1['Lat'], 0)]

        # Create the polygon
        pol = kml.newpolygon(name='', outerboundaryis=coords)

        # Set the altitude mode for the polygon
        pol.altitudemode = simplekml.AltitudeMode.absolute
        pol.extrude = 1

    lin = kml.newlinestring(name="Path", altitudemode=simplekml.AltitudeMode.absolute)
    lin.coords = line_coords 

    return kml

def write_kmz(kml, output_file):
    # Create a new Kml object
    new_kml = simplekml.Kml()
    new_kml.name = kml.name
    new_kml.document.name = kml.name
    # Create folders for the air and ground points and paths
    air_point_folder = new_kml.newfolder(name='Air Points')
    #ground_point_folder = new_kml.newfolder(name='Ground Points')
    air_path_folder = new_kml.newfolder(name='Air Path')
    #ground_path_folder = new_kml.newfolder(name='Ground Path')
    curtain_folder = new_kml.newfolder(name='Curtain')

    # Set styling for line
    line_style = simplekml.Style()
    line_style.linestyle.color = simplekml.Color.blue  # Example color for the line

    # Set styling for polygon
    polygon_style = simplekml.Style()
    polygon_style.polystyle.color = simplekml.Color.red  # Example color for the line

    # Set styling for points
    point_style = simplekml.Style()
    point_style.iconstyle.icon.href = 'http://maps.google.com/mapfiles/kml/shapes/placemark_circle.png'  # Simple point icon
    point_style.iconstyle.icon.scale = 0.1
    point_style.labelstyle.scale = 0.01  # No label for the point

    # Apply style to line and add to line folder
    for feature in kml.features:
        if isinstance(feature, simplekml.LineString):
            coords_str = feature.coords.__str__()  # Get string representation of coordinates
            coords = [tuple(map(float, coord_str.split(','))) for coord_str in coords_str.split(' ')]  # Convert string to list of tuples of floats
            line = air_path_folder.newlinestring(coords=coords) # Create new LineString with these coordinates
            line.style = line_style
            line.altitudemode = simplekml.AltitudeMode.absolute

        elif isinstance(feature, simplekml.Point):
            coords_str = feature.coords.__str__()  # Get string representation of coordinates
            coords = tuple(map(float, coords_str.split(',')))  # Convert string to tuple of floats
            point = air_point_folder.newpoint(coords=[coords]) # Create new Point with these coordinates
            point.style = point_style
            point.altitudemode = simplekml.AltitudeMode.absolute
            point.description = feature.description

        elif isinstance(feature, simplekml.Polygon):
            coords_str = feature.outerboundaryis.coords.__str__()
            coords_list = coords_str.split(" ")
            coords = [(float(coord.split(",")[0]), float(coord.split(",")[1]), float(coord.split(",")[2])) for coord in coords_list]
            polygon = curtain_folder.newpolygon(outerboundaryis=coords) # Create new Polygon with these coordinates
            polygon.style = polygon_style
            polygon.altitudemode = simplekml.AltitudeMode.absolute
            polygon.extrude = 1


    # Save KMZ
    new_kml.savekmz(output_file)
    new_kml.save("test.kml")

def extract_data2(filename):
    # Open the binary file
    dfreader = DFReader_binary(filename)
    start_timestamp = dfreader.clock.timebase
    end_timestamp = dfreader.clock.timestamp
    # Convert the timestamp to a datetime object
    start_datetime = datetime.utcfromtimestamp(start_timestamp)

    tf = TimezoneFinder()

    # Get the timezone
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
    
    display_name = (f"{filename}: {date}")
    # Create KML and write KMZ
    kml = create_kml(data, display_name, fieldnames_str, start_timestamp, end_timestamp)
    # Create the output filename
    
    write_kmz(kml, f"{filename}.kmz")
    print("Count: ", count)



# Use the function
filename = "9.bin"
extract_data2(filename)

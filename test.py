import sys
from telemetry_data_extractor import DataExtractor
from kml_creator import KMLCreator
from kmz_writer import KMZWriter


# Use the classes
filename = "10.bin"
extractor = DataExtractor(filename)
extratorData = extractor.extract_data()
if extratorData == None:
    sys.exit(0)

data, date, fieldnames, timestart, timeend = extratorData
display_name = (f"{filename}: {date}")
creator = KMLCreator(data, display_name, fieldnames, timestart, timeend)
kml = creator.create_kml()
filename =f"{filename}.kmz"
writer = KMZWriter(kml, filename)
writer.write_kmz()
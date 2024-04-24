import simplekml

class KMZWriter:
    def __init__(self, kml, output_file):
        self.kml = kml
        self.output_file = output_file

    def write_kmz(self):
        # Create a new Kml object
        new_kml = simplekml.Kml()
        new_kml.name = self.kml.name
        new_kml.document.name = self.kml.name
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
        for feature in self.kml.features:
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
        new_kml.savekmz(self.output_file)
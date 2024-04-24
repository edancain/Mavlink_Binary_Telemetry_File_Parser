import simplekml

class KMLCreator:
    def __init__(self, data, display_name, fieldnames, timestart, timeend):
        self.data = data
        self.filename = display_name
        self.fieldnames = fieldnames
        self.timestart = timestart
        self.timeend = timeend

    def create_kml(self):
        kml = simplekml.Kml()
        kml.name = self.filename
        kml.document.name = self.filename
        
        # Create a list to store coordinates for line string
        line_coords = []

        for entry in self.data:
            # If entry is already a dictionary, use it directly
            if isinstance(entry, dict):
                entry_dict = entry
            else:
                # Create a dictionary from fieldnames and entry
                entry_dict = dict(zip(self.fieldnames, entry))

            # Add altitude to the coordinates
            coords_with_alt = (entry_dict['Lng'], entry_dict['Lat'], entry_dict['Alt'])

            # Add point with altitude
            pnt = kml.newpoint(name='',
                            coords=[coords_with_alt],
                            description='\n'.join(f'{key}: {value}' for key, value in entry_dict.items()))
            pnt.altitudemode = simplekml.AltitudeMode.absolute
            line_coords.append(coords_with_alt)

        for i in range(len(self.data) - 1):
            entry_dict1 = self.data[i]
            entry_dict2 = self.data[i+1]

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
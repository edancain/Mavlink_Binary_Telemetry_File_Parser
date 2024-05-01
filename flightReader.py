import gzip
import requests
from io import BytesIO


# https://www.flightreader.com/api/documentation/

def main():
    # Replace with your Flight Reader API secret key
    secret_api_key = "4UWX2CGEHDA9CWHQ9Y4L"

    # Set the DJI account credentials
    email = "edan.cain@gmail.com"
    password = "AirspaceLink2024"

    form_data = {
        "email": email,
        "password": password
    }

    headers = {
        "Accept-Encoding": "gzip",
        "Authorization": f"Bearer {secret_api_key}"
    }

    response = requests.post("https://api.flightreader.com/v1/accounts/dji", data=form_data, headers=headers)

    if response.ok:
        response_bytes = response.content

        if "gzip" in response.headers.get("Content-Encoding", ""):
            with gzip.open(BytesIO(response_bytes), "rb") as compressed_stream:
                decompressed_bytes = compressed_stream.read()
        else:
            decompressed_bytes = response_bytes

        response_text = decompressed_bytes.decode("utf-8")
        print(response_text)
    else:
        print(f"Error: {response.status_code}")

if __name__ == "__main__":
    main()

import requests

# URL to upload the file
url = "https://www.phantomhelp.com/logviewer/upload/UploadFile.asp"

# File to upload
file_path = "/path/to/DJIFlightRecord_2018-10-22_[12-20-51]log.zip"

# Parameters for the request
params = {
    "response": "03AFcWeA7FOFwkpQz39ZEDGNbin4snE3hJPFlsHA2-nQuqcVZC5C3SznYYawpKunTYcXxrMmoYi77OEumrv3cp6SBVTWpbmujn3L0c-nSfBtLU5hgseZY-h9AeGksHGN-KafwnYf2qgpcV-00uqKt7u2fDIRqK3w15SLxtkLt492YBbebYqWOSWa2A7dx7LytGZOhitSODt8YiIqjMW5-vk1BQRURWcvvjtKCpZHHWSmkO_R_oaYB8-mec1dOJMMNCCfnttrkj8pZ5qK4IBSpUqIuczCsSdCvV4CbvNEFDgrW1UzP4W2ijWmsHphx3h66BrMLBCTD4UrGy5eAHo7nHonASzTH-a_3g3827XrECi_OKVmD0sC8VJO8CUMdqSoEeIP0wQXGQylxT_cmCjxu5oya42a7j2Cxxb7TTWia-cL6IwqCv7QafjbB8keiKNVAP8BmSsu3yoO18OljfqrHz50AIK9B5JDiKyk6QGlPOBXYHzrDYLGVscipoQI56pLPqkxjuspypJcqAnAtTamGdnOmq-rAqLMy5ZH-8T8LrmPV8Vt_6ep1mQuTIbUWiuL5IYFft79qxUnMD0-38_gnXBFTxAEZp48W1f_PvzgnODWGRSQJkDEiVfx-yPcGWe5gY-YmAEozsN5-QQlTGrGRe0YP_boiM9-9ywOYs69W8WRB1vmLoO83D2yq5SwZ4uAvUb5ioj5Rkc6y3i5TXqTQacqO9HORzYRw_MlrB9AsSYLBf6EM6l36gNi8",
    "email": ""
}

# Files to upload
files = {"file": open(file_path, "rb")}

# Send POST request with files and parameters
response = requests.post(url, files=files, params=params)

# Print response
print("Status Code:", response.status_code)
print("Redirected URL:", response.url)
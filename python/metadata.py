import os, requests

_METADATA_URL = "http://metadata.google.internal/computeMetadata/v1/"
_METADATA_HEADERS = {"Metadata-Flavor": "Google"}
_REGION_ID = "instance/region"
_PROJECT_ID = "project/project-id"

_project_id = os.getenv('GOOGLE_CLOUD_PROJECT')
_region_id = os.getenv('LOCATION_ID')

def _retrieve_metadata_server(metadata_key, timeout=5):
    """Retrieve the metadata key in the metadata server.

    See: https://cloud.google.com/compute/docs/storing-retrieving-metadata

    Args:
        metadata_key (str): Key of the metadata which will form the url.
        timeout (number): number of seconds to wait for the HTTP request

    Returns:
        str: The value of the metadata key returned by the metadata server.
    """
    url = _METADATA_URL + metadata_key
    try:
        response = requests.get(url, headers=_METADATA_HEADERS, timeout=timeout)
        if response.status_code == requests.codes.ok:
            return response.text
    except requests.exceptions.RequestException:
        # Ignore the exception, connection failed means the attribute does not
        # exist in the metadata server.
        pass
    return ''

def resource_project():
    global _project_id
    if not _project_id:
        _project_id = _retrieve_metadata_server(_PROJECT_ID)
    return _project_id

def resource_region():
    global _region_id
    if not _region_id:
        s = _retrieve_metadata_server(_REGION_ID)
        last = s.rfind('/')
        if last >= 0 and last +1 <= len(s):
            _region_id = s[last+1:]
        _region_id = s
    return _region_id
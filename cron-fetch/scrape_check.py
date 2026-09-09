"""Smoke test: verify requests + beautifulsoup4 are working."""
import requests
from bs4 import BeautifulSoup

resp = requests.get("https://example.com", timeout=10)
resp.raise_for_status()

soup = BeautifulSoup(resp.text, "html.parser")
print("status:", resp.status_code)
print("page title:", soup.title.string)

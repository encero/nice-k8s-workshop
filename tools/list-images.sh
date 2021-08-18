#!/bin/bash

curl -H "Authorization: Bearer $DO_TOKEN" "https://api.digitalocean.com/v2/images?per_page=200&type=distribution" | jq

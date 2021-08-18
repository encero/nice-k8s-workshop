#!/bin/bash

curl -H "Authorization: Bearer $DO_TOKEN" "https://api.digitalocean.com/v2/account/keys?per_page=200" | jq

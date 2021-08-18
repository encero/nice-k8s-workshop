#!/bin/bash

curl -H "Authorization: Bearer $DO_TOKEN" https://api.digitalocean.com/v2/regions | jq

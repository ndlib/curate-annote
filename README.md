Curate-Annote
=============

This repository holds code for a prototype service looking at ways to integrate institutional repositories with other services.
It is unlinkely to be of value to others outside this organization.


## Running elastic search on development machine

  docker pull docker.elastic.co/elasticsearch/elasticsearch:7.5.1
  docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" elasticsearch:7.5.1

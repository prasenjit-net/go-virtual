// Runs once on first container start (docker-entrypoint-initdb.d).
// By the time this script executes mongod is up but the replica set may not
// be initiated yet — the healthcheck handles rs.initiate(), so we just ensure
// the database exists by creating a placeholder document and removing it.
db = db.getSiblingDB("go-virtual");
db.createCollection("_init");
db.getCollection("_init").drop();

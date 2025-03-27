#!/bin/bash

ADMIN_ID=$(psql -t -A -q -c "INSERT INTO users (name, email, password_hash, activated) VALUES ('admin', 'admin11@example.com', decode('24326124313224464e64506f59767048776a6d594447614c2e584d714f466972556c554a78635137366c75475841426777502e47516a50463141534f', 'hex'), true) RETURNING id::INTEGER;" ${GREENLIGHT_DB_DSN})

echo "value (${ADMIN_ID})"

psql -c "INSERT INTO users_permissions (user_id, permission_id) VALUES (${ADMIN_ID}, 1),(${ADMIN_ID}, 2),(${ADMIN_ID}, 3);" ${GREENLIGHT_DB_DSN}

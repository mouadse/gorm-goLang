# Fitness Tracker Postman Collection

Files:

- `fitness-tracker-seeded.postman_collection.json`
- `fitness-tracker-local.postman_environment.json`

Import both files into Postman, select the `Fitness Tracker API - Local Seeded` environment, then run the collection from the top.

The collection assumes the local API is running at `http://localhost:8082` and the seed command has been run. It logs in with the seeded admin account:

- Email: `alex@example.com`
- Password: `password123`

The login request stores `accessToken`, `refreshToken`, and `userId` as collection variables. List/create requests also capture seeded or generated IDs so later requests can execute without manual copy-paste.

The `Manual and Destructive` folder is included for endpoint coverage but is disabled by default. It contains endpoints that need a real TOTP/recovery code, require an available external service or local file, use websocket upgrade behavior, revoke sessions, or delete account/user data.

// The suite drives the API directly for setup and teardown, including admin/reset. That
// used to point at a hardcoded localhost:9096, so running the suite against any other
// instance still aimed the reset at whatever happened to be on that port. Ask once, here.
export const API_BASE = process.env.WARDEN_API_BASE ?? API_BASE;

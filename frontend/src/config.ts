// There's no login flow — `api` has no auth yet (documented gap, see
// docs/PROJECT_PLAN.md). Every acknowledge/resolve action is attributed to
// this single fixed user (infra/postgres/init.sql seeds it with this
// exact id) rather than the frontend pretending to know who's "logged
// in".
export const ACTING_USER_ID =
  import.meta.env.VITE_ACTING_USER_ID ?? "33333333-0000-0000-0000-000000000001";

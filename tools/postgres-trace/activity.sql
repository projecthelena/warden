-- Run from a separate psql client connected to the database under investigation.
-- Output contains no SQL text or bind parameters. Keep Warden's db_trace logs too.
\pset tuples_only on
\pset format unaligned
SELECT json_build_object(
 'at',clock_timestamp(),
 'activity',coalesce((SELECT json_agg(json_build_object(
  'pid',pid,'backend_start',backend_start,'backend_type',backend_type,
  'state',state,'query_start',query_start,'xact_start',xact_start,
  'state_change',state_change,'backend_xid',backend_xid,'query_id',query_id,
  'trace',substring(query FROM '^/\*warden:([A-Za-z0-9_-]+:[0-9]+)\*/'),
  'wait_type',wait_event_type,'wait',wait_event,
  'blockers',pg_blocking_pids(pid),
  'locks',(SELECT coalesce(json_agg(json_build_object(
    'type',locktype,'mode',mode,'granted',granted,'relation_oid',relation,
    'page',page,'tuple',tuple,'transactionid',transactionid,'waitstart',waitstart
  )),'[]'::json) FROM pg_locks WHERE pg_locks.pid=a.pid AND NOT granted)
 )) FROM pg_stat_activity a
 WHERE datname=current_database() AND pid<>pg_backend_pid()
 AND state IS DISTINCT FROM 'idle'),'[]'::json)
)::jsonb;
\watch 0.25

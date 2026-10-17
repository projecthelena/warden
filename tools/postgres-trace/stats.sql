\pset tuples_only on
\pset format unaligned
SELECT json_build_object(
 'at',clock_timestamp(),
 'wal',(SELECT row_to_json(w) FROM pg_stat_wal w),
 'checkpointer',(SELECT row_to_json(c) FROM pg_stat_checkpointer c),
 'bgwriter',(SELECT row_to_json(b) FROM pg_stat_bgwriter b),
 'io',(SELECT json_agg(i) FROM pg_stat_io i),
 'database',(SELECT row_to_json(d) FROM pg_stat_database d WHERE datname=current_database())
)::jsonb;
\watch 1

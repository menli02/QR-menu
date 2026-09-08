# Migration runner: golang-migrate plus this repo's SQL, as one versioned,
# immutable artifact.
#
# Why an image rather than mounting the SQL from a ConfigMap:
#
#   - kustomize refuses to read files outside its own directory, so a
#     generator would need either a copy of the SQL under deploy/k8s (a
#     second place for it to drift) or --load-restrictor LoadRestrictionsNone
#     on every invocation (a flag every caller has to remember).
#   - A generator also needs each migration listed by name, which goes
#     stale the moment someone adds one — and the failure is silent: the
#     new migration simply never runs.
#   - The deploy pipeline already promotes image tags. The schema is part
#     of a release, so it should be promoted the same way and roll back
#     the same way.
#
# Built and pushed by .github/workflows/docker.yml alongside the services.
FROM migrate/migrate:v4.19.1

# Whole directories, so adding a migration needs no change anywhere else —
# unlike a ConfigMap generator, which needs each file listed by name and
# silently skips the one somebody forgot to add.
#
# Paths are repo-root relative because the build context is the repo root,
# matching the four service Dockerfiles.
COPY migrations/catalog   /migrations/catalog
COPY migrations/order     /migrations/order
COPY migrations/identity  /migrations/identity

# migrate/migrate sets its own ENTRYPOINT; deploy/k8s/migrate-job.yaml
# overrides both command and args to run all three databases in sequence.
USER 65532:65532

# Specializing this base worker

Create a clean independent copy rather than an inheritance layer or implicit
link to a sibling worker.

1. Select a reviewed commit of the Bench library containing `enterprise-architect`.
   Git commits version the source; worker metadata supplies lifecycle, owner,
   requirements and the approved file inventory.
2. From that checkout, obtain the full commit and use the existing repository
   exporter into a new destination outside the checkout:

       git rev-parse HEAD
       python3 scripts/workers export enterprise-architect /absolute/new-destination \
         --ref FULL_COMMIT --allow-experimental

   Use `--allow-experimental` only when the selected library entry is actually
   experimental. The clean export contains `expert/` and `worker.lock.json`.
3. Preserve `worker.lock.json` unchanged as provenance for the source copy. It
   is not runtime instruction and its original hashes do not claim to describe
   later amendments.
4. Create a separate authoring workspace and copy only the exported `expert/`
   into it as `expert/`; keep the original export and lock unchanged. Use Hire
   against this authoring workspace to amend the definition. Update `PROFILE.md`
   with the bounded domain, add or revise only
   genuinely needed Brief skills, and retain the base artifact contract and
   checker behavior:

       hire build -C /absolute/specialist-authoring \
         -evidence /absolute/specialist-evidence -- \
         'Specialize the pinned Enterprise Architect copy for DOMAIN while
          retaining its base artifact contract and independent evaluation.'

5. Give the specialist its own library ID, version, owner, reviewed inventory,
   tests, and evaluation evidence. Re-run the base synthetic checker suite plus
   domain-specific positive and negative cases, Brief lint, and `hire verify`.
   Promote it through normal review and make a new pinned export if reusable.

A profile change intentionally changes `profile_sha256`; every subsequent job
must bind the new exact bytes. Do not add a loader, inheritance engine, worker
registry, implicit sibling reads, bundled job artifacts, credentials, memory,
provider SDK, runner, or orchestration. Teams pass selected artifacts explicitly
to this specialist in a fresh workspace and evaluate it independently.

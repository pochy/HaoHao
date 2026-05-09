# Repository Agent Notes

## Search Discipline

- Use `RIPGREP_CONFIG_PATH=` for `rg` commands in this repository to avoid noise from a missing local ripgrep config.
- Start with exact symbols, route names, query names, and likely directories. Avoid broad first-pass searches such as `rg drive` across the whole repository.
- When debugging Drive, prefer the `haohao-drive-debug` skill workflow before expanding the search scope.
- If a narrow command produces the needed fact, do not run a broader command afterward just for orientation.

## Drive Debugging

- For "uploaded but not visible" reports, check DB row existence, workspace/folder, API response, authorization filtering, and frontend filters in that order.
- Always consider SQL `ORDER BY` plus `LIMIT` before assuming upload, OpenFGA, or frontend rendering failure.
- Do not paste full Drive list JSON into the conversation; extract filenames, counts, or specific fields.
- After a Drive debug session, update `haohao-drive-debug` if the fastest successful path differs from the skill.

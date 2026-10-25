# Protocol: mods

## API (emulator and app; bearer = a website `/api/v1` token)

| Route | What |
| --- | --- |
| `GET /api/v1/titles/{title_id}/mods` | manifest of published mods: id, version, sha256, size, licence, summary, package_url |
| `GET /api/v1/mods/{id}` | one entry |
| `GET /api/v1/mods/{id}/package` | the zip; `ETag` is the sha256, honours `If-None-Match`; a replaced version stays for 30 days |
| `POST /api/v1/submissions` (bearer) | multipart field `package`: the zip. Refused (422, with the reason) unless it passes the gate |
| `GET /api/v1/me/submissions` (bearer) | your submissions and their status |
| `GET /api/v1/me/favourites`, `PUT`/`DELETE /api/v1/me/favourites/{id}` (bearer) | favourites |

## Package format (MOD-01)

A zip with `mod.json` at the root and every file under `romfs/` or `exefs/`:

```json
{"id":"blue-hat","name":"Blue Hat","version":"1.0","title_id":"0100000000010000",
 "author":"someone","licence":"CC-BY-4.0","source_url":"https://example.org/blue-hat",
 "layout":"romfs","summary":"A blue hat."}
```

The gate (MOD-02) refuses packages without a licence or source, with files outside the layout,
or containing game data, executables or keys (`.nca .nsp .xci .nso .npdm .kip .tik .cert`,
`*.keys`, `exefs/main`). Manual review always follows.

## Curation (`Authorization: Bearer $MODS_INTERNAL_KEY`)

```sh
curl -H "Authorization: Bearer $KEY" http://127.0.0.1:20030/internal/queue
curl -X POST -H "Authorization: Bearer $KEY" http://127.0.0.1:20030/internal/mods/$ID/publish
curl -X POST -H "Authorization: Bearer $KEY" -d '{"reason":"..."}' http://127.0.0.1:20030/internal/mods/$ID/reject
curl -X POST -H "Authorization: Bearer $KEY" -d '{"reason":"..."}' http://127.0.0.1:20030/internal/mods/$ID/takedown
```

Publishing retires the previously published version of the same mod (30-day grace for
downloads); takedown removes every version at once.

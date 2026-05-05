# druid-mcp

A Go-based MCP (Model Context Protocol) server for querying and inserting data into Druid sandbox environments.

## Install

```bash
go install github.com/BthaBro/druid-mcp/cmd/druid-mcp@latest
```

Or build from source:

```bash
git clone <repo-url>
cd druid-mcp
go build -o bin/druid-mcp ./cmd/druid-mcp
```

## Configuration

### 1. Create your config file

```bash
mkdir -p ~/.config/druid-mcp
cp config.example.yaml ~/.config/druid-mcp/config.yaml
```

Edit `~/.config/druid-mcp/config.yaml` to add your environments and auth credentials:

```yaml
environments:
  sb5:
    url: https://druid.sb5.example.com:443
    auth: "Basic dXNlcjpwYXNz"
  sb1:
    url: https://druid.sb1.example.com
    auth: "Basic dXNlcjpwYXNz"
```

The `auth` value is the full `Authorization` HTTP header value.

### 2. Configure OpenCode

Add the MCP server to your `opencode.json` configuration. Create one entry per environment you want to use:

```json
{
  "mcp": {
    "druid-mcp-sb5": {
      "type": "local",
      "enabled": true,
      "command": ["druid-mcp"],
      "environment": { "DRUID_ENV": "sb5" }
    },
    "druid-mcp-sb1": {
      "type": "local",
      "enabled": true,
      "command": ["druid-mcp"],
      "environment": { "DRUID_ENV": "sb1" }
    }
  }
}
```

The `DRUID_ENV` variable tells the server which environment from your config to use.

> **Note:** Make sure `$GOPATH/bin` (usually `~/go/bin`) is on your `$PATH` so that the `druid-mcp` command is found.
> If you prefer an absolute path, use `"command": ["/path/to/go/bin/druid-mcp"]`.

## Tools

| Tool | Description |
|------|-------------|
| `druid_query` | Execute read-only SELECT queries |
| `druid_ingest` | Execute INSERT/REPLACE queries (with confirmation) |
| `druid_task_status` | Check ingestion task status |
| `druid_cancel_task` | Cancel a running ingestion task |
| `druid_list_environments` | List all configured environments |

### druid_query

```
query: SQL SELECT statement
```

Guardrails prevent write operations from being executed through this tool.

### druid_ingest

```
query: SQL INSERT INTO or REPLACE INTO statement
confirmed: boolean (default: false)
```

- First call without `confirmed` shows a preview
- Call again with `confirmed: true` to execute
- Requires `PARTITIONED BY` clause
- Returns a task ID for async tracking

### druid_task_status

```
task_id: The task ID returned by druid_ingest
```

### druid_cancel_task

```
task_id: The task ID to cancel
```

## Druid SQL Reference

### SELECT queries

```sql
SELECT * FROM "sb5-campaign-postback" WHERE __time = '2026-05-04'
```

### INSERT (append)

```sql
INSERT INTO "sb5-campaign-postback"
SELECT TIMESTAMP '2026-05-04' AS "__time", ...
FROM (SELECT 1)
PARTITIONED BY DAY
```

### REPLACE (overwrite partition)

```sql
REPLACE INTO "sb5-campaign-postback"
OVERWRITE WHERE __time = '2026-05-04'
SELECT ...
FROM "sb5-campaign-postback"
WHERE __time = '2026-05-04'
PARTITIONED BY DAY
```

### Multi-row INSERT with EXTERN

```sql
INSERT INTO "sb5-campaign-postback"
SELECT TIME_PARSE("ts") AS "__time", ...
FROM TABLE(
  EXTERN(
    '{"type":"inline","data":"<json-lines>"}',
    '{"type":"json"}',
    '[{"name":"ts","type":"string"}, ...]'
  )
)
PARTITIONED BY DAY
```

### Schema discovery

```sql
SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES
SELECT COLUMN_NAME, DATA_TYPE FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME = 'my-table'
```

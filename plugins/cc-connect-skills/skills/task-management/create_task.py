#!/usr/bin/env python3
"""Create a Feishu Bitable task record after reading the table schema."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from typing import Any


STATUS_DEFAULTS = {
    "Action": "Not started",
    "Status": "Queue",
}

TITLE_FIELDS = ("Task", "Task Name")
CONTENT_FIELDS = ("Relevant content", "Description")
GROUP_FIELDS = ("Related Group", "Task Group")
CREATE_TIME_FIELDS = ("Create Time", "Create time")


def run_bytedcli(args: list[str]) -> dict[str, Any]:
    result = subprocess.run(
        ["bytedcli", "--json", "feishu", "bitable", *args],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or result.stdout.strip())
    data = json.loads(result.stdout)
    if data.get("status") != "success":
        raise RuntimeError(json.dumps(data, ensure_ascii=False))
    return data


def field_names(app_token: str, table_id: str) -> set[str]:
    data = run_bytedcli([
        "field",
        "list",
        "--app-token",
        app_token,
        "--table-id",
        table_id,
    ])
    return {field["field_name"] for field in data["data"].get("fields", [])}


def first_present(names: tuple[str, ...], existing: set[str]) -> str | None:
    return next((name for name in names if name in existing), None)


def build_fields(args: argparse.Namespace, existing: set[str]) -> dict[str, Any]:
    fields: dict[str, Any] = {}

    title_field = first_present(TITLE_FIELDS, existing)
    if not title_field:
        raise RuntimeError(f"No supported title field found. Expected one of: {', '.join(TITLE_FIELDS)}")
    fields[title_field] = args.task

    content_field = first_present(CONTENT_FIELDS, existing)
    if content_field and args.content:
        fields[content_field] = args.content

    if "workspace" in existing and args.workspace:
        fields["workspace"] = args.workspace

    for status_field, default_value in STATUS_DEFAULTS.items():
        if status_field in existing:
            fields[status_field] = args.status or default_value
            break

    group_field = first_present(GROUP_FIELDS, existing)
    if group_field and args.chat_id:
        fields[group_field] = [{"id": args.chat_id}]

    create_time_field = first_present(CREATE_TIME_FIELDS, existing)
    if create_time_field:
        fields[create_time_field] = int(time.time() * 1000)

    return fields


def create_record(app_token: str, table_id: str, fields: dict[str, Any]) -> dict[str, Any]:
    return run_bytedcli([
        "record",
        "create",
        "--app-token",
        app_token,
        "--table-id",
        table_id,
        "--body-json",
        json.dumps({"fields": fields}, ensure_ascii=False),
    ])


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Create a Bitable task record using the table schema.")
    parser.add_argument("--app-token", required=True)
    parser.add_argument("--table-id", required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--task", required=True)
    parser.add_argument("--content", default="")
    parser.add_argument("--chat-id", default="")
    parser.add_argument("--status", default="")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        existing = field_names(args.app_token, args.table_id)
        fields = build_fields(args, existing)
        result = create_record(args.app_token, args.table_id, fields)
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    record_id = result.get("data", {}).get("record", {}).get("record_id") or result.get("data", {}).get("record_id")
    print(json.dumps({"record_id": record_id, "fields": fields, "raw": result.get("data")}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

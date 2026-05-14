# Tasks

## Purpose

Manage task records.

Tasks are data records. The SDK should help systems create, read, update, delete,
and subscribe to task records. It should not decide how a system executes,
schedules, or queues those tasks.

## Method Families

- create
- get
- list
- update
- delete

## Notes

Do not add task execution helpers.

Do not add queue management helpers.

Systems using the SDK decide what to do with task records they receive. The SDK
only surfaces task data and keeps subscribed task views fresh.

Task subscriptions are covered in `sync.md`.

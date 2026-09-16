#!/bin/bash

set -e

BACKUP_DIR="/home/delica/upgraded-guacamole/backups"
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")

mkdir -p "$BACKUP_DIR/daily"
mkdir -p "$BACKUP_DIR/weekly"
mkdir -p "$BACKUP_DIR/monthly"

backup() {
    local type="$1"

    echo "Creating $type backup..."

    docker exec guac pg_dump \
        -U postgres \
        -d guac \
        -Fc \
        > "$BACKUP_DIR/$type/$TIMESTAMP.dump"
}

# Always create a daily backup
backup "daily"

# On Sunday, also create a weekly backup
if [ "$(date +%u)" -eq 7 ]; then
    backup "weekly"
fi

# On the first day of the month, also create a monthly backup
if [ "$(date +%d)" -eq "01" ]; then
    backup "monthly"
fi

# Keep newest 7 daily backups
ls -t "$BACKUP_DIR/daily/"*.dump 2>/dev/null |
    tail -n +8 |
    xargs -r rm --

# Keep newest 4 weekly backups
ls -t "$BACKUP_DIR/weekly/"*.dump 2>/dev/null |
    tail -n +5 |
    xargs -r rm --

# Keep newest 6 monthly backups
ls -t "$BACKUP_DIR/monthly/"*.dump 2>/dev/null |
    tail -n +7 |
    xargs -r rm --

echo "Backup complete."

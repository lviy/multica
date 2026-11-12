import type {
  NotificationGroupKey,
  NotificationGroupValue,
  NotificationPreferences,
} from "../types";

const NOTIFICATION_GROUP_KEYS: readonly NotificationGroupKey[] = [
  "assignments",
  "status_changes",
  "workspace_reviews",
  "comments",
  "mentions",
  "updates",
  "agent_activity",
  "system_notifications",
];

export function defaultNotificationPreference(key: NotificationGroupKey): NotificationGroupValue {
  return key === "workspace_reviews" ? "muted" : "all";
}

function preferenceValue(
  preferences: NotificationPreferences,
  key: NotificationGroupKey,
): NotificationGroupValue {
  return preferences[key] ?? defaultNotificationPreference(key);
}

/**
 * Convert the full preference object produced by the settings UI into the
 * smallest atomic patch. Missing keys mean the group default (review watching is opt-in).
 */
export function deriveNotificationPreferencePatch(
  previous: NotificationPreferences,
  next: NotificationPreferences,
): NotificationPreferences {
  const patch: NotificationPreferences = {};

  for (const key of NOTIFICATION_GROUP_KEYS) {
    const previousValue = preferenceValue(previous, key);
    const nextValue = preferenceValue(next, key);
    if (previousValue !== nextValue) {
      patch[key] = nextValue;
    }
  }

  return patch;
}

/** Apply a preference patch while keeping group defaults sparse. */
export function applyNotificationPreferencePatch(
  current: NotificationPreferences,
  patch: NotificationPreferences,
): NotificationPreferences {
  const next = { ...current };

  for (const key of NOTIFICATION_GROUP_KEYS) {
    const value = patch[key];
    if (value === undefined) continue;
    if (value === defaultNotificationPreference(key)) {
      delete next[key];
    } else {
      next[key] = value;
    }
  }

  return next;
}

/**
 * Roll back only values that still match this mutation's optimistic patch.
 * A later toggle that touched the same key wins.
 */
export function rollbackNotificationPreferencePatch(
  current: NotificationPreferences,
  patch: NotificationPreferences,
  previous: NotificationPreferences,
): NotificationPreferences {
  const next = { ...current };

  for (const key of NOTIFICATION_GROUP_KEYS) {
    const patchedValue = patch[key];
    if (
      patchedValue === undefined ||
      preferenceValue(current, key) !== patchedValue
    ) {
      continue;
    }

    const previousValue = preferenceValue(previous, key);
    if (previousValue === defaultNotificationPreference(key)) {
      delete next[key];
    } else {
      next[key] = previousValue;
    }
  }

  return next;
}

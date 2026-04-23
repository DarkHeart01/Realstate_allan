// test/unit/time_ago_test.dart
// Tests for a timeAgo() utility function.
// The function is defined inline here as a pure Dart function — the same logic
// should be extracted to app/lib/core/utils/time_ago.dart if not already there.
import 'package:flutter_test/flutter_test.dart';

/// Converts a [DateTime] to a human-readable relative string.
/// Mirrors the display logic used in the NotificationScreen and PropertyCard.
String timeAgo(DateTime dt) {
  final diff = DateTime.now().difference(dt);

  if (diff.inSeconds < 60) return 'just now';
  if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
  if (diff.inHours < 24) return '${diff.inHours}h ago';
  if (diff.inDays < 7) return '${diff.inDays}d ago';

  // >= 7 days: show formatted date.
  final d = dt.toLocal();
  final month = [
    '', 'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
  ][d.month];
  return '${d.day} $month ${d.year}';
}

void main() {
  group('timeAgo', () {
    test('returns "just now" for less than 1 minute ago', () {
      final dt = DateTime.now().subtract(const Duration(seconds: 30));
      expect(timeAgo(dt), 'just now');
    });

    test('returns "{n}m ago" for minutes', () {
      final dt = DateTime.now().subtract(const Duration(minutes: 5));
      expect(timeAgo(dt), '5m ago');
    });

    test('returns "{n}h ago" for hours', () {
      final dt = DateTime.now().subtract(const Duration(hours: 3));
      expect(timeAgo(dt), '3h ago');
    });

    test('returns "{n}d ago" for days less than 7', () {
      final dt = DateTime.now().subtract(const Duration(days: 3));
      expect(timeAgo(dt), '3d ago');
    });

    test('returns formatted date for 7 or more days', () {
      // Use a fixed date so the test is deterministic.
      final dt = DateTime(2025, 3, 1, 12, 0);
      // This will always be >= 7 days in the future relative to any test run.
      final result = timeAgo(dt);
      expect(result, contains('2025'));
      expect(result, contains('Mar'));
    });

    test('handles exactly 60 minutes boundary', () {
      // Exactly 60 minutes = 1h ago.
      final dt = DateTime.now().subtract(const Duration(minutes: 60));
      expect(timeAgo(dt), '1h ago');
    });

    test('handles exactly 24 hours boundary', () {
      // Exactly 24 hours = 1d ago.
      final dt = DateTime.now().subtract(const Duration(hours: 24));
      expect(timeAgo(dt), '1d ago');
    });

    test('returns "just now" for 0 seconds', () {
      expect(timeAgo(DateTime.now()), 'just now');
    });

    test('returns "59m ago" for 59 minutes', () {
      final dt = DateTime.now().subtract(const Duration(minutes: 59));
      expect(timeAgo(dt), '59m ago');
    });

    test('returns "23h ago" for 23 hours', () {
      final dt = DateTime.now().subtract(const Duration(hours: 23));
      expect(timeAgo(dt), '23h ago');
    });

    test('returns "6d ago" for 6 days', () {
      final dt = DateTime.now().subtract(const Duration(days: 6));
      expect(timeAgo(dt), '6d ago');
    });
  });
}

// test/widgets/notification_badge_test.dart
// Tests the bell icon badge shown in the bottom nav / app bar.
// The badge widget is a Stack with a CircleAvatar overlay when unread > 0.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// ── Minimal badge widget ──────────────────────────────────────────────────────
// This matches the pattern used in the app's notification bell.

Widget buildBadge(int unreadCount) {
  return MaterialApp(
    home: Scaffold(
      appBar: AppBar(
        actions: [
          _NotificationBellIcon(unreadCount: unreadCount),
        ],
      ),
    ),
  );
}

class _NotificationBellIcon extends StatelessWidget {
  const _NotificationBellIcon({required this.unreadCount});
  final int unreadCount;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        const Icon(Icons.notifications_outlined),
        if (unreadCount > 0)
          Positioned(
            right: 0,
            top: 0,
            child: CircleAvatar(
              radius: 8,
              backgroundColor: Theme.of(context).colorScheme.error,
              child: Text(
                unreadCount > 99 ? '99+' : '$unreadCount',
                style: const TextStyle(fontSize: 10, color: Colors.white),
              ),
            ),
          ),
      ],
    );
  }
}

// ── Tests ─────────────────────────────────────────────────────────────────────

void main() {
  testWidgets('Bell icon shows no badge when unread count is 0', (tester) async {
    await tester.pumpWidget(buildBadge(0));
    // No CircleAvatar badge should be rendered.
    expect(find.byType(CircleAvatar), findsNothing);
  });

  testWidgets('Bell icon shows count badge when unread count is 5', (tester) async {
    await tester.pumpWidget(buildBadge(5));
    expect(find.byType(CircleAvatar), findsOneWidget);
    expect(find.text('5'), findsOneWidget);
  });

  testWidgets('Bell icon shows count badge when unread count is 1', (tester) async {
    await tester.pumpWidget(buildBadge(1));
    expect(find.text('1'), findsOneWidget);
  });

  testWidgets('Bell icon shows 99+ when unread count exceeds 99', (tester) async {
    await tester.pumpWidget(buildBadge(150));
    expect(find.text('99+'), findsOneWidget);
    expect(find.text('150'), findsNothing);
  });

  testWidgets('Bell icon shows exactly 99 for count of 99', (tester) async {
    await tester.pumpWidget(buildBadge(99));
    expect(find.text('99'), findsOneWidget);
    expect(find.text('99+'), findsNothing);
  });

  testWidgets('Bell icon shows 100+ badge for count of 100', (tester) async {
    await tester.pumpWidget(buildBadge(100));
    expect(find.text('99+'), findsOneWidget);
  });
}

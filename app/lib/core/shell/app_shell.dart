// app/lib/core/shell/app_shell.dart
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Scaffold that wraps the three primary tabs (Listings / Map / Calculator)
/// and provides a persistent [BottomNavigationBar].
///
/// This widget is the `builder` of a [StatefulShellRoute] — GoRouter passes
/// the currently active branch child via [navigationShell].
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  static const _destinations = [
    NavigationDestination(
      icon: Icon(Icons.list_alt_outlined),
      selectedIcon: Icon(Icons.list_alt),
      label: 'Listings',
    ),
    NavigationDestination(
      icon: Icon(Icons.map_outlined),
      selectedIcon: Icon(Icons.map),
      label: 'Map',
    ),
    NavigationDestination(
      icon: Icon(Icons.calculate_outlined),
      selectedIcon: Icon(Icons.calculate),
      label: 'Calculator',
    ),
    NavigationDestination(
      icon: Icon(Icons.notifications_outlined),
      selectedIcon: Icon(Icons.notifications),
      label: 'Activity',
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: navigationShell,
      bottomNavigationBar: NavigationBar(
        selectedIndex: navigationShell.currentIndex,
        onDestinationSelected: (index) {
          navigationShell.goBranch(
            index,
            // Re-tap on the current tab scrolls back to the root of that branch.
            initialLocation: index == navigationShell.currentIndex,
          );
        },
        destinations: _destinations,
      ),
    );
  }
}

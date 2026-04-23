// app/lib/core/router/router.dart
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../shell/app_shell.dart';
import '../../features/calculator/screens/calculator_screen.dart';
import '../../features/map/screens/map_screen.dart';
import '../../features/notifications/screens/notification_screen.dart';
import '../../features/properties/screens/add_property_screen.dart';
import '../../features/properties/screens/property_detail_screen.dart';
import '../../features/properties/screens/property_list_screen.dart';

abstract class AppRoutes {
  static const String home = '/';
  static const String login = '/login';
  static const String register = '/register';
  static const String properties = '/properties';
  static const String propertyDetail = '/properties/:id';
  static const String addProperty = '/properties/add';
  static const String editProperty = '/properties/:id/edit';
  static const String map = '/map';
  static const String calculator = '/calculator';
  static const String notifications = '/notifications';
}

final GoRouter appRouter = GoRouter(
  debugLogDiagnostics: true,
  initialLocation: AppRoutes.properties,
  routes: [
    // Auth routes — outside the shell
    GoRoute(
      path: AppRoutes.login,
      name: 'login',
      builder: (context, state) => const _PlaceholderScreen(title: 'Login'),
    ),
    GoRoute(
      path: AppRoutes.register,
      name: 'register',
      builder: (context, state) => const _PlaceholderScreen(title: 'Register'),
    ),

    // ── Shell with three bottom-nav branches ─────────────────────────────────
    StatefulShellRoute.indexedStack(
      builder: (context, state, navigationShell) =>
          AppShell(navigationShell: navigationShell),
      branches: [
        // Branch 0 — Listings
        StatefulShellBranch(
          routes: [
            GoRoute(
              path: AppRoutes.properties,
              name: 'properties',
              builder: (context, state) => const PropertyListScreen(),
              routes: [
                GoRoute(
                  path: 'add',
                  name: 'addProperty',
                  builder: (context, state) => const AddPropertyScreen(),
                ),
                GoRoute(
                  path: ':id',
                  name: 'propertyDetail',
                  builder: (context, state) {
                    final id = state.pathParameters['id']!;
                    return PropertyDetailScreen(propertyId: id);
                  },
                  routes: [
                    GoRoute(
                      path: 'edit',
                      name: 'editProperty',
                      builder: (context, state) {
                        final id = state.pathParameters['id']!;
                        return AddPropertyScreen(editPropertyId: id);
                      },
                    ),
                  ],
                ),
              ],
            ),
          ],
        ),

        // Branch 1 — Map
        StatefulShellBranch(
          routes: [
            GoRoute(
              path: AppRoutes.map,
              name: 'map',
              builder: (context, state) => const MapScreen(),
            ),
          ],
        ),

        // Branch 2 — Calculator
        StatefulShellBranch(
          routes: [
            GoRoute(
              path: AppRoutes.calculator,
              name: 'calculator',
              builder: (context, state) => const CalculatorScreen(),
            ),
          ],
        ),

        // Branch 3 — Activity / Notifications
        StatefulShellBranch(
          routes: [
            GoRoute(
              path: AppRoutes.notifications,
              name: 'notifications',
              builder: (context, state) => const NotificationScreen(),
            ),
          ],
        ),
      ],
    ),
  ],
);

class _PlaceholderScreen extends StatelessWidget {
  const _PlaceholderScreen({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Center(child: Text(title)),
    );
  }
}

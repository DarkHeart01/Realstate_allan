import 'package:flutter_riverpod/flutter_riverpod.dart';
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
  static const String properties = '/properties';
  static const String propertyDetail = '/properties/:id';
  static const String addProperty = '/properties/add';
  static const String editProperty = '/properties/:id/edit';
  static const String map = '/map';
  static const String calculator = '/calculator';
  static const String notifications = '/notifications';
}

// Auth is handled by the MaterialApp builder in main.dart.
// This router only manages authenticated routes inside the shell.
final routerProvider = Provider<GoRouter>((ref) {
  final router = GoRouter(
    debugLogDiagnostics: true,
    initialLocation: AppRoutes.properties,
    routes: [
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

  ref.onDispose(router.dispose);
  return router;
});

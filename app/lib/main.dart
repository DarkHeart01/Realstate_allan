// app/lib/main.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/date_symbol_data_local.dart';

import 'core/router/router.dart';
import 'core/theme/theme.dart';
import 'features/auth/providers/auth_provider.dart';
import 'features/auth/screens/login_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await initializeDateFormatting('en_IN');
  runApp(
    const ProviderScope(
      child: RealEstateApp(),
    ),
  );
}

class RealEstateApp extends ConsumerWidget {
  const RealEstateApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    final authState = ref.watch(authProvider);

    return MaterialApp.router(
      title: 'RealEstate Platform',
      debugShowCheckedModeBanner: false,
      theme: buildLightTheme(),
      darkTheme: buildDarkTheme(),
      themeMode: ThemeMode.system,
      routerConfig: router,
      builder: (context, child) {
        // Show splash while checking stored token on cold start.
        if (authState.isInitializing) {
          return const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          );
        }
        // Overlay LoginScreen directly — no GoRouter navigation needed.
        // When isLoggedIn becomes true the builder re-runs and shows child.
        if (!authState.isLoggedIn) {
          return const LoginScreen();
        }
        return child ?? const SizedBox.shrink();
      },
    );
  }
}

// app/lib/core/theme/theme.dart
import 'package:flutter/material.dart';

// Brand colours
const Color _primaryColor = Color(0xFF1A56DB);     // professional blue
const Color _secondaryColor = Color(0xFF0E9F6E);   // success green
const Color _errorColor = Color(0xFFE02424);
const Color _surfaceColor = Color(0xFFF9FAFB);

/// Returns the app's light ThemeData.
ThemeData buildLightTheme() {
  final colorScheme = ColorScheme.fromSeed(
    seedColor: _primaryColor,
    secondary: _secondaryColor,
    error: _errorColor,
    surface: _surfaceColor,
    brightness: Brightness.light,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: colorScheme,
    scaffoldBackgroundColor: _surfaceColor,
    appBarTheme: AppBarTheme(
      backgroundColor: colorScheme.primary,
      foregroundColor: colorScheme.onPrimary,
      elevation: 0,
    ),
    cardTheme: CardThemeData(
      elevation: 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: Colors.white,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(8),
        borderSide: BorderSide(color: colorScheme.outline),
      ),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: colorScheme.primary,
        foregroundColor: colorScheme.onPrimary,
        minimumSize: const Size.fromHeight(48),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
    ),
  );
}

/// Returns the app's dark [ThemeData].
ThemeData buildDarkTheme() {
  final colorScheme = ColorScheme.fromSeed(
    seedColor: _primaryColor,
    secondary: _secondaryColor,
    error: _errorColor,
    brightness: Brightness.dark,
  );

  return ThemeData(
    useMaterial3: true,
    colorScheme: colorScheme,
    cardTheme: CardThemeData(
      elevation: 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        backgroundColor: colorScheme.primary,
        foregroundColor: colorScheme.onPrimary,
        minimumSize: const Size.fromHeight(48),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
    ),
  );
}

// test/unit/indian_format_test.dart
// Mirrors backend FormatIndianNumber behaviour using the intl package.
// The PropertyCard uses NumberFormat.currency(locale: 'en_IN', ...) —
// these tests verify the Flutter output matches the backend expected values.
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/intl.dart';

// Helper that matches the PropertyCard formatting.
String formatIndianPrice(double price) {
  final fmt = NumberFormat.currency(
    locale: 'en_IN',
    symbol: '₹',
    decimalDigits: 0,
  );
  return fmt.format(price);
}

void main() {
  group('Indian number format', () {
    test('formats hundreds correctly', () {
      expect(formatIndianPrice(999), '₹999');
    });

    test('formats thousands correctly', () {
      expect(formatIndianPrice(10000), '₹10,000');
    });

    test('formats lakhs correctly', () {
      // 1,00,000
      expect(formatIndianPrice(100000), '₹1,00,000');
    });

    test('formats 50 lakhs correctly', () {
      // 50,00,000
      expect(formatIndianPrice(5000000), '₹50,00,000');
    });

    test('formats crores correctly', () {
      // 1,00,00,000
      expect(formatIndianPrice(10000000), '₹1,00,00,000');
    });

    test('formats 10 crores correctly', () {
      expect(formatIndianPrice(100000000), '₹10,00,00,000');
    });

    test('formats zero', () {
      expect(formatIndianPrice(0), '₹0');
    });

    test('formats 1500 correctly', () {
      expect(formatIndianPrice(1500), '₹1,500');
    });
  });
}

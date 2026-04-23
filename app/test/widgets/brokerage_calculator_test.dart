// test/widgets/brokerage_calculator_test.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:realestate_app/features/calculator/providers/calculator_provider.dart';
import 'package:realestate_app/features/calculator/screens/calculator_screen.dart';

// ── Fake notifier ─────────────────────────────────────────────────────────────

class _FakeCalcNotifier extends StateNotifier<CalculatorState> {
  _FakeCalcNotifier(super.initial);

  @override
  Future<void> calculate() async {}
  void setMode(CalcMode m) => state = state.copyWith(mode: m);
  void setPropertyValue(double v) => state = state.copyWith(propertyValue: v);
  void setMonthlyRent(double v) => state = state.copyWith(monthlyRent: v);
  void setCommissionRate(double v) => state = state.copyWith(commissionRate: v);
  void setSplitRatio(String v) => state = state.copyWith(splitRatio: v);
}

Widget _buildScreen(CalculatorState initial) {
  return ProviderScope(
    overrides: [
      calculatorProvider.overrideWith((_) => _FakeCalcNotifier(initial)),
    ],
    child: const MaterialApp(home: CalculatorScreen()),
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

void main() {
  testWidgets('CalculatorScreen shows SALE mode by default', (tester) async {
    await tester.pumpWidget(_buildScreen(const CalculatorState()));
    expect(find.text('Sale'), findsAtLeastNWidgets(1));
    expect(find.text('Property Value (₹)'), findsOneWidget);
  });

  testWidgets('CalculatorScreen hides monthly rent in SALE mode', (tester) async {
    await tester.pumpWidget(_buildScreen(const CalculatorState(mode: CalcMode.sale)));
    expect(find.text('Monthly Rent (₹)'), findsNothing);
  });

  testWidgets('CalculatorScreen shows monthly rent in RENTAL mode', (tester) async {
    await tester.pumpWidget(_buildScreen(const CalculatorState(mode: CalcMode.rental)));
    expect(find.text('Monthly Rent (₹)'), findsOneWidget);
    expect(find.text('Property Value (₹)'), findsNothing);
  });

  testWidgets('CalculatorScreen shows inline error (not SnackBar) for invalid split',
      (tester) async {
    const errorMsg = 'split_ratio: A + B must equal 100';
    await tester.pumpWidget(
        _buildScreen(const CalculatorState(error: errorMsg)));
    // Error must appear as inline text, not a SnackBar.
    expect(find.text(errorMsg), findsOneWidget);
    expect(find.byType(SnackBar), findsNothing);
  });

  testWidgets('CalculatorScreen shows total commission in result card', (tester) async {
    await tester.pumpWidget(_buildScreen(const CalculatorState(
      totalCommission: 100000,
      totalCommissionFormatted: '₹1,00,000',
    )));
    expect(find.text('₹1,00,000'), findsOneWidget);
  });

  testWidgets('CalculatorScreen shows party A and B splits in result card', (tester) async {
    await tester.pumpWidget(_buildScreen(CalculatorState(
      totalCommission: 100000,
      totalCommissionFormatted: '₹1,00,000',
      splitA: 50000,
      splitB: 50000,
      splitAFormatted: '₹50,000',
      splitBFormatted: '₹50,000',
    )));
    expect(find.text('₹50,000'), findsAtLeastNWidgets(2));
  });

  testWidgets('CalculatorScreen shows shimmer/loading indicator while loading',
      (tester) async {
    await tester.pumpWidget(
        _buildScreen(const CalculatorState(isLoading: true)));
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });
}

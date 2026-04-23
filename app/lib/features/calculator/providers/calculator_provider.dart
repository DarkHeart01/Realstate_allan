// app/lib/features/calculator/providers/calculator_provider.dart
import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_client.dart';

// ── Providers ─────────────────────────────────────────────────────────────────

final _dioProvider = Provider<Dio>((ref) => createDioClient());

final calculatorProvider =
    StateNotifierProvider<CalculatorNotifier, CalculatorState>((ref) {
  final dio = ref.watch(_dioProvider);
  return CalculatorNotifier(dio);
});

// ── State ─────────────────────────────────────────────────────────────────────

enum CalcMode { sale, rental }

class CalculatorState {
  final CalcMode mode;
  final double propertyValue;
  final double monthlyRent;
  final double commissionRate;
  final String splitRatio; // e.g. "60:40"

  final bool isLoading;
  final String? error;

  // Result fields — null until a successful calculation.
  final double? totalCommission;
  final String? totalCommissionFormatted;
  final double? splitA;
  final double? splitB;
  final String? splitAFormatted;
  final String? splitBFormatted;

  const CalculatorState({
    this.mode = CalcMode.sale,
    this.propertyValue = 0,
    this.monthlyRent = 0,
    this.commissionRate = 2.0,
    this.splitRatio = '',
    this.isLoading = false,
    this.error,
    this.totalCommission,
    this.totalCommissionFormatted,
    this.splitA,
    this.splitB,
    this.splitAFormatted,
    this.splitBFormatted,
  });

  CalculatorState copyWith({
    CalcMode? mode,
    double? propertyValue,
    double? monthlyRent,
    double? commissionRate,
    String? splitRatio,
    bool? isLoading,
    String? error,
    double? totalCommission,
    String? totalCommissionFormatted,
    double? splitA,
    double? splitB,
    String? splitAFormatted,
    String? splitBFormatted,
    bool clearResult = false,
    bool clearError = false,
  }) {
    return CalculatorState(
      mode: mode ?? this.mode,
      propertyValue: propertyValue ?? this.propertyValue,
      monthlyRent: monthlyRent ?? this.monthlyRent,
      commissionRate: commissionRate ?? this.commissionRate,
      splitRatio: splitRatio ?? this.splitRatio,
      isLoading: isLoading ?? this.isLoading,
      error: clearError ? null : (error ?? this.error),
      totalCommission: clearResult ? null : (totalCommission ?? this.totalCommission),
      totalCommissionFormatted:
          clearResult ? null : (totalCommissionFormatted ?? this.totalCommissionFormatted),
      splitA: clearResult ? null : (splitA ?? this.splitA),
      splitB: clearResult ? null : (splitB ?? this.splitB),
      splitAFormatted: clearResult ? null : (splitAFormatted ?? this.splitAFormatted),
      splitBFormatted: clearResult ? null : (splitBFormatted ?? this.splitBFormatted),
    );
  }

  bool get hasResult => totalCommission != null;
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class CalculatorNotifier extends StateNotifier<CalculatorState> {
  CalculatorNotifier(this._dio) : super(const CalculatorState());

  final Dio _dio;
  Timer? _debounce;

  void setMode(CalcMode mode) {
    state = state.copyWith(mode: mode, clearResult: true, clearError: true);
    _scheduleCalculate();
  }

  void setPropertyValue(double v) {
    state = state.copyWith(propertyValue: v, clearResult: true, clearError: true);
    _scheduleCalculate();
  }

  void setMonthlyRent(double v) {
    state = state.copyWith(monthlyRent: v, clearResult: true, clearError: true);
    _scheduleCalculate();
  }

  void setCommissionRate(double v) {
    state = state.copyWith(commissionRate: v, clearResult: true, clearError: true);
    _scheduleCalculate();
  }

  void setSplitRatio(String v) {
    state = state.copyWith(splitRatio: v, clearResult: true, clearError: true);
    _scheduleCalculate();
  }

  void _scheduleCalculate() {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), calculate);
  }

  Future<void> calculate() async {
    final s = state;
    final value =
        s.mode == CalcMode.sale ? s.propertyValue : s.monthlyRent;
    if (value <= 0) return;

    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final body = <String, dynamic>{
        'mode': s.mode == CalcMode.sale ? 'SALE' : 'RENTAL',
        'commission_rate': s.commissionRate,
      };
      if (s.mode == CalcMode.sale) {
        body['property_value'] = s.propertyValue;
      } else {
        body['monthly_rent'] = s.monthlyRent;
      }
      if (s.splitRatio.isNotEmpty) {
        body['split_ratio'] = s.splitRatio;
      }

      final response = await _dio.post('/api/tools/calculator', data: body);
      final data = response.data['data'] as Map<String, dynamic>;

      state = state.copyWith(
        isLoading: false,
        totalCommission: (data['total_commission'] as num).toDouble(),
        totalCommissionFormatted: data['total_commission_formatted'] as String?,
        splitA: (data['split_a'] as num?)?.toDouble(),
        splitB: (data['split_b'] as num?)?.toDouble(),
        splitAFormatted: data['split_a_formatted'] as String?,
        splitBFormatted: data['split_b_formatted'] as String?,
      );
    } on DioException catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.response?.data?['message'] as String? ?? e.message,
      );
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  @override
  void dispose() {
    _debounce?.cancel();
    super.dispose();
  }
}

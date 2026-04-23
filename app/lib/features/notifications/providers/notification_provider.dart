// app/lib/features/notifications/providers/notification_provider.dart
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_client.dart';
import '../models/notification_job.dart';

// ── Providers ─────────────────────────────────────────────────────────────────

final _dioProvider = Provider<Dio>((ref) => createDioClient());

final notificationProvider =
    StateNotifierProvider<NotificationNotifier, NotificationState>((ref) {
  return NotificationNotifier(ref.watch(_dioProvider));
});

// ── State ─────────────────────────────────────────────────────────────────────

class NotificationState {
  final List<NotificationJob> jobs;
  final bool isLoading;
  final bool isLoadingMore;
  final String? error;
  final int total;
  final int offset;
  static const int limit = 50;

  const NotificationState({
    this.jobs = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.error,
    this.total = 0,
    this.offset = 0,
  });

  bool get hasMore => offset + jobs.length < total;

  NotificationState copyWith({
    List<NotificationJob>? jobs,
    bool? isLoading,
    bool? isLoadingMore,
    String? error,
    int? total,
    int? offset,
  }) {
    return NotificationState(
      jobs: jobs ?? this.jobs,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      error: error,
      total: total ?? this.total,
      offset: offset ?? this.offset,
    );
  }
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class NotificationNotifier extends StateNotifier<NotificationState> {
  NotificationNotifier(this._dio) : super(const NotificationState());

  final Dio _dio;

  Future<void> refresh() async {
    state = state.copyWith(isLoading: true, offset: 0, error: null);
    await _fetch(replace: true);
  }

  Future<void> fetchNext() async {
    if (state.isLoadingMore || !state.hasMore) return;
    state = state.copyWith(isLoadingMore: true);
    await _fetch(replace: false);
  }

  Future<void> _fetch({required bool replace}) async {
    try {
      final response = await _dio.get('/api/notifications', queryParameters: {
        'limit': NotificationState.limit,
        'offset': replace ? 0 : state.offset,
      });

      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List<dynamic>)
          .map((e) => NotificationJob.fromJson(e as Map<String, dynamic>))
          .toList();
      final pagination = data['pagination'] as Map<String, dynamic>;
      final total = pagination['count'] as int;
      final newOffset =
          replace ? items.length : state.offset + items.length;

      state = state.copyWith(
        jobs: replace ? items : [...state.jobs, ...items],
        total: total,
        offset: newOffset,
        isLoading: false,
        isLoadingMore: false,
      );
    } on DioException catch (e) {
      state = state.copyWith(
        isLoading: false,
        isLoadingMore: false,
        error: e.response?.data?['message'] as String? ?? e.message,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        isLoadingMore: false,
        error: e.toString(),
      );
    }
  }

  /// SUPER_ADMIN: trigger a stale listing scan on the server.
  Future<String> triggerStaleScan() async {
    final response =
        await _dio.post('/api/admin/notifications/scan-stale');
    final data = response.data as Map<String, dynamic>;
    final found = data['data']?['stale_found'] as int? ?? 0;
    final queued = data['data']?['queued'] as int? ?? 0;
    // Refresh the list after triggering.
    refresh();
    return 'Found $found stale listings, queued $queued notifications.';
  }
}

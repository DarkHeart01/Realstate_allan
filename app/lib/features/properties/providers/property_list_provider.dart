// app/lib/features/properties/providers/property_list_provider.dart
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_client.dart';
import '../models/property.dart';

// ── Filter params ─────────────────────────────────────────────────────────────

class FilterParams {
  final String? category;
  final String? propertyType;
  final double? minPrice;
  final double? maxPrice;
  final double? minArea;
  final bool? isDirectOwner;
  final List<String> tags;

  const FilterParams({
    this.category,
    this.propertyType,
    this.minPrice,
    this.maxPrice,
    this.minArea,
    this.isDirectOwner,
    this.tags = const [],
  });

  FilterParams copyWith({
    String? category,
    String? propertyType,
    double? minPrice,
    double? maxPrice,
    double? minArea,
    bool? isDirectOwner,
    List<String>? tags,
  }) {
    return FilterParams(
      category: category ?? this.category,
      propertyType: propertyType ?? this.propertyType,
      minPrice: minPrice ?? this.minPrice,
      maxPrice: maxPrice ?? this.maxPrice,
      minArea: minArea ?? this.minArea,
      isDirectOwner: isDirectOwner ?? this.isDirectOwner,
      tags: tags ?? this.tags,
    );
  }

  Map<String, dynamic> toQueryParams() {
    final p = <String, dynamic>{};
    if (category != null) p['category'] = category;
    if (propertyType != null) p['type'] = propertyType;
    if (minPrice != null) p['min_price'] = minPrice.toString();
    if (maxPrice != null) p['max_price'] = maxPrice.toString();
    if (minArea != null) p['min_area'] = minArea.toString();
    if (isDirectOwner != null) p['is_direct_owner'] = isDirectOwner.toString();
    if (tags.isNotEmpty) p['tags'] = tags.join(',');
    return p;
  }
}

// ── State ─────────────────────────────────────────────────────────────────────

class PropertyListState {
  final List<Property> properties;
  final FilterParams filters;
  final int offset;
  final int limit;
  final int total;
  final bool isLoading;
  final bool isLoadingMore;
  final String? error;

  const PropertyListState({
    this.properties = const [],
    this.filters = const FilterParams(),
    this.offset = 0,
    this.limit = 20,
    this.total = 0,
    this.isLoading = false,
    this.isLoadingMore = false,
    this.error,
  });

  bool get hasMore => offset + properties.length < total;

  PropertyListState copyWith({
    List<Property>? properties,
    FilterParams? filters,
    int? offset,
    int? limit,
    int? total,
    bool? isLoading,
    bool? isLoadingMore,
    String? error,
  }) {
    return PropertyListState(
      properties: properties ?? this.properties,
      filters: filters ?? this.filters,
      offset: offset ?? this.offset,
      limit: limit ?? this.limit,
      total: total ?? this.total,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      error: error,
    );
  }
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class PropertyListNotifier extends StateNotifier<PropertyListState> {
  PropertyListNotifier() : super(const PropertyListState());

  final _dio = createDioClient();

  Future<void> refresh() async {
    state = state.copyWith(isLoading: true, offset: 0, error: null);
    await _fetch(replace: true);
  }

  Future<void> fetchNext() async {
    if (state.isLoadingMore || !state.hasMore) return;
    state = state.copyWith(isLoadingMore: true);
    await _fetch(replace: false);
  }

  void applyFilters(FilterParams filters) {
    state = state.copyWith(filters: filters, offset: 0);
    refresh();
  }

  Future<void> deleteProperty(String id) async {
    await _dio.delete('/api/properties/$id');
    state = state.copyWith(
      properties: state.properties.where((p) => p.id != id).toList(),
      total: state.total > 0 ? state.total - 1 : 0,
    );
  }

  Future<void> _fetch({required bool replace}) async {
    if (kDebugMode && const bool.fromEnvironment('USE_MOCK_DATA', defaultValue: false)) {
      await Future.delayed(const Duration(milliseconds: 500));
      final mock = _mockProperties();
      state = state.copyWith(
        properties: replace ? mock : [...state.properties, ...mock],
        total: mock.length,
        offset: replace ? mock.length : state.offset + mock.length,
        isLoading: false,
        isLoadingMore: false,
      );
      return;
    }

    try {
      final queryParams = {
        'limit': state.limit.toString(),
        'offset': replace ? '0' : state.offset.toString(),
        ...state.filters.toQueryParams(),
      };

      final response = await _dio.get('/api/properties', queryParameters: queryParams);
      final data = response.data as Map<String, dynamic>;
      final items = (data['data'] as List<dynamic>)
          .map((e) => Property.fromJson(e as Map<String, dynamic>))
          .toList();
      final pagination = data['pagination'] as Map<String, dynamic>;
      final total = pagination['count'] as int;
      final newOffset = replace ? items.length : state.offset + items.length;

      state = state.copyWith(
        properties: replace ? items : [...state.properties, ...items],
        total: total,
        offset: newOffset,
        isLoading: false,
        isLoadingMore: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        isLoadingMore: false,
        error: e.toString(),
      );
    }
  }

  List<Property> _mockProperties() {
    return [
      Property(
        id: 'mock-1',
        listingCategory: 'SELLING',
        propertyType: 'FLAT',
        ownerName: 'Demo Owner',
        ownerContact: '9999999999',
        price: 5000000,
        builtUpArea: 1200,
        locationLat: 19.0760,
        locationLng: 72.8777,
        tags: const ['Hot Lead', 'Aditya_Lead'],
        isDirectOwner: true,
        createdBy: 'mock-user',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ),
    ];
  }
}

// ── Provider ──────────────────────────────────────────────────────────────────

final propertyListProvider =
    StateNotifierProvider<PropertyListNotifier, PropertyListState>(
  (ref) => PropertyListNotifier(),
);

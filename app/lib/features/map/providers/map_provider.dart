// app/lib/features/map/providers/map_provider.dart
import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart';

import '../../../core/network/dio_client.dart';
import '../../properties/models/property.dart';
import '../../properties/providers/property_list_provider.dart';

// ── Providers ─────────────────────────────────────────────────────────────────

final _dioProvider = Provider<Dio>((ref) => createDioClient());

/// Shared filter state between list and map tabs.
final mapFilterProvider = StateProvider<FilterParams>((ref) => const FilterParams());

final mapProvider = StateNotifierProvider<MapNotifier, MapState>((ref) {
  final dio = ref.watch(_dioProvider);
  return MapNotifier(dio);
});

// ── State ─────────────────────────────────────────────────────────────────────

class MapState {
  final List<Property> properties;
  final bool isLoading;
  final String? error;

  const MapState({
    this.properties = const [],
    this.isLoading = false,
    this.error,
  });

  MapState copyWith({
    List<Property>? properties,
    bool? isLoading,
    String? error,
  }) {
    return MapState(
      properties: properties ?? this.properties,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

// ── Notifier ──────────────────────────────────────────────────────────────────

class MapNotifier extends StateNotifier<MapState> {
  MapNotifier(this._dio) : super(const MapState());

  final Dio _dio;
  Timer? _debounce;

  /// Called on `onCameraIdle` — debounced 400 ms to avoid hammering the API
  /// while the user is still panning.
  void loadForBounds(LatLngBounds bounds, FilterParams filters) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 400), () {
      _fetch(bounds, filters);
    });
  }

  Future<void> _fetch(LatLngBounds bounds, FilterParams filters) async {
    state = state.copyWith(isLoading: true, error: null);

    try {
      final sw = bounds.southwest;
      final ne = bounds.northeast;
      final boundsParam =
          '${sw.latitude},${sw.longitude},${ne.latitude},${ne.longitude}';

      final params = <String, dynamic>{
        'bounds': boundsParam,
        'limit': 200,
        ...filters.toQueryParams(),
      };

      final response = await _dio.get('/api/properties', queryParameters: params);
      final data = response.data as Map<String, dynamic>;
      final list = (data['data'] as List<dynamic>)
          .map((e) => Property.fromJson(e as Map<String, dynamic>))
          .toList();

      state = state.copyWith(properties: list, isLoading: false);
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

// ── Clustering helper ─────────────────────────────────────────────────────────

/// Converts a list of [Property] into a set of [Marker]s.
/// At low zoom levels (< 12) properties within the same ~0.01° grid cell are
/// collapsed into a single count marker.
Set<Marker> buildMarkers({
  required List<Property> properties,
  required double zoom,
  required void Function(Property) onTap,
}) {
  if (zoom >= 12) {
    return properties.map((p) => _singleMarker(p, onTap)).toSet();
  }

  // Simple grid clustering.
  final grid = <String, List<Property>>{};
  const cell = 0.05; // ~5 km
  for (final p in properties) {
    final key =
        '${(p.locationLat / cell).floor()}:${(p.locationLng / cell).floor()}';
    grid.putIfAbsent(key, () => []).add(p);
  }

  return grid.entries.map((entry) {
    final cluster = entry.value;
    if (cluster.length == 1) return _singleMarker(cluster.first, onTap);

    final lat =
        cluster.map((p) => p.locationLat).reduce((a, b) => a + b) / cluster.length;
    final lng =
        cluster.map((p) => p.locationLng).reduce((a, b) => a + b) / cluster.length;

    return Marker(
      markerId: MarkerId('cluster_${entry.key}'),
      position: LatLng(lat, lng),
      infoWindow: InfoWindow(title: '${cluster.length} properties'),
    );
  }).toSet();
}

Marker _singleMarker(Property p, void Function(Property) onTap) {
  // SELLING = coral (red), BUYING = teal
  final hue = p.listingCategory == 'SELLING'
      ? BitmapDescriptor.hueRed
      : BitmapDescriptor.hueCyan;

  return Marker(
    markerId: MarkerId(p.id),
    position: LatLng(p.locationLat, p.locationLng),
    icon: BitmapDescriptor.defaultMarkerWithHue(hue),
    infoWindow: InfoWindow(
      title: p.propertyType,
      snippet: p.priceFormatted,
    ),
    onTap: () => onTap(p),
  );
}

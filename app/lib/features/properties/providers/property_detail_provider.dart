// app/lib/features/properties/providers/property_detail_provider.dart
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/dio_client.dart';
import '../models/property.dart';

final propertyDetailProvider =
    FutureProvider.family<Property, String>((ref, id) async {
  final dio = createDioClient();
  final response = await dio.get('/api/properties/$id');
  final data = (response.data as Map<String, dynamic>)['data'] as Map<String, dynamic>;
  return Property.fromJson(data);
});

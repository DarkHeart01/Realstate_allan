import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

const _kAccessTokenKey = 'access_token';

/// Creates and configures the application's [Dio] HTTP client.
///
/// [onUnauthorized] is called when any request receives a 401 response —
/// use this to trigger logout from a Riverpod notifier.
Dio createDioClient({VoidCallback? onUnauthorized}) {
  const baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080',
  );

  final dio = Dio(
    BaseOptions(
      baseUrl: baseUrl,
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 30),
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
      },
    ),
  );

  dio.interceptors.addAll([
    AuthInterceptor(onUnauthorized: onUnauthorized),
    LogInterceptor(requestBody: true, responseBody: true),
  ]);

  return dio;
}

/// Attaches the stored Bearer token to outgoing requests and auto-logs out
/// on 401 (expired / invalid token).
class AuthInterceptor extends Interceptor {
  AuthInterceptor({this.onUnauthorized});

  final VoidCallback? onUnauthorized;
  final _storage = const FlutterSecureStorage();

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    final token = await _storage.read(key: _kAccessTokenKey);
    if (token != null && token.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    if (err.response?.statusCode == 401) {
      _storage.delete(key: _kAccessTokenKey);
      onUnauthorized?.call();
    }
    handler.next(err);
  }
}

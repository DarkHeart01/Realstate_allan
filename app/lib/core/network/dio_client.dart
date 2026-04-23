// app/lib/core/network/dio_client.dart
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Key used to store the JWT access token in secure storage.
const _kAccessTokenKey = 'access_token';

/// Creates and configures the application's [Dio] HTTP client.
///
/// - Base URL is read from the compile-time `API_BASE_URL` dart-define.
/// - The [AuthInterceptor] attaches the stored JWT to every request.
/// - In Phase 2 the interceptor will handle 401 → token refresh automatically.
Dio createDioClient() {
  const baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080', // Android emulator → host machine
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
    AuthInterceptor(dio),
    LogInterceptor(requestBody: true, responseBody: true),
  ]);

  return dio;
}

/// Attaches the stored Bearer token to outgoing requests.
/// Phase 2 will add automatic token refresh on 401.
class AuthInterceptor extends Interceptor {
  AuthInterceptor(Dio dio);

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

  // TODO Phase 2: implement onError to detect 401 and refresh the access token
  // transparently before retrying the original request.
}

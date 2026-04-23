// app/lib/features/notifications/models/notification_job.dart

class NotificationJob {
  final String id;
  final String? propertyId;
  final String jobType;
  final String status;
  final String? twilioSid;
  final DateTime? scheduledAt;
  final DateTime? sentAt;
  final DateTime createdAt;

  const NotificationJob({
    required this.id,
    this.propertyId,
    required this.jobType,
    required this.status,
    this.twilioSid,
    this.scheduledAt,
    this.sentAt,
    required this.createdAt,
  });

  factory NotificationJob.fromJson(Map<String, dynamic> json) {
    return NotificationJob(
      id: json['id'] as String,
      propertyId: json['property_id'] as String?,
      jobType: json['job_type'] as String,
      status: json['status'] as String,
      twilioSid: json['twilio_sid'] as String?,
      scheduledAt: json['scheduled_at'] != null
          ? DateTime.parse(json['scheduled_at'] as String)
          : null,
      sentAt: json['sent_at'] != null
          ? DateTime.parse(json['sent_at'] as String)
          : null,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// Display-friendly label for the job type.
  String get typeLabel {
    switch (jobType) {
      case 'STALE_LISTING':
        return 'Stale Listing';
      case 'FOLLOW_UP':
        return 'Follow-Up';
      case 'DEAL_ALERT':
        return 'Deal Alert';
      default:
        return jobType;
    }
  }

  /// Display-friendly status label.
  String get statusLabel {
    switch (status) {
      case 'PENDING':
        return 'Pending';
      case 'SENT':
        return 'Sent';
      case 'FAILED':
        return 'Failed';
      default:
        return status;
    }
  }
}

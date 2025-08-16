#!/usr/bin/env python3
"""
Production Gmail Automation System for B2B Logistics Coordination with PostgreSQL Database

Architecture:
- PostgreSQL Database: Replaces file-based storage
- Cron Job: Fetches logistics requests from appointments_scheduling table
- Push Notifications: Real-time reply detection via Google Cloud Pub/Sub
- AI Response Generator: Uses Gemini 1.5 Flash for logistics coordination responses
- Thread Safety: Database-level concurrency control

Use Case:
- B2B Logistics SaaS platform coordinating with courier partners
- Automated shipping slot requests based on TAT (Turn Around Time) requirements
- AI-powered responses to courier partner availability and capacity updates
- Seamless integration with TMS (Transportation Management System) and courier APIs

Database Tables:
- appointments_scheduling: Scheduled logistics coordination requests
- appointment_email_details: Sent email tracking for logistics communications
- processed_replies: Duplicate prevention for courier partner replies
"""

import json
import os
import time
import threading
import base64
import hashlib
import traceback
import psycopg2
import psycopg2.extras
from datetime import datetime, timedelta
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from typing import Dict, List, Optional, Set
from dataclasses import dataclass, asdict
from flask import Flask, request, jsonify
import logging
import re
from email.message import EmailMessage
import uuid

# Load environment variables
try:
    from dotenv import load_dotenv
    load_dotenv()
    print("[OK] Environment variables loaded")
except ImportError:
    print("[INFO] python-dotenv not installed, using system environment")

try:
    from google.auth.transport.requests import Request
    from google.oauth2.credentials import Credentials
    from googleapiclient.discovery import build
    import google.generativeai as genai
    from google.cloud import pubsub_v1
    print("[OK] All required libraries imported successfully")
except ImportError as e:
    print(f"[ERROR] Missing required packages: {e}")
    print("Install with: pip install google-auth google-auth-oauthlib google-api-python-client google-generativeai google-cloud-pubsub flask python-dotenv psycopg2-binary")
    exit(1)

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('gmail_automation.log', encoding='utf-8'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

# Database configuration
DB_CONFIG = {
    "host": "database-openleaf-1.cyuh3uofyiy4.ap-south-1.rds.amazonaws.com",
    "dbname": "b2b-productiondb",
    "user": "openleaf",
    "password": "geralt1212leaf",
    "port": 5432
}

@dataclass
class AppointmentScheduling:
    """Represents a scheduled appointment from appointments_scheduling table"""
    user_id: str
    schedule_datetime: datetime
    is_processed: str  # 'Y' or 'N'
    email_id: str
    order_id: Optional[str] = None  # UUID for order tracking
    po_ids: Optional[str] = None # Comma-separated PO IDs
    cc_email_id: Optional[str] = None # Comma-separated CC emails
    phase: Optional[int] = None  # Phase identifier: 1=logistics, 2=carrier emails
    carrier_email_id: Optional[str] = None  # Carrier email column for phase 2
    carrier_cc_email_id: Optional[str] = None  # Carrier CC email column for phase 2
    is_reminder: Optional[bool] = None  # Boolean flag for reminder emails
    confirmed_appointment_time: Optional[datetime] = None  # From order_activity JOIN
    address: Optional[str] = None  # Delivery address from orders table
    id: Optional[int] = None  # Auto-generated primary key

@dataclass
class SentEmail:
    """Track sent emails for reply monitoring - stored in appointment_email_details"""
    campaign_id: str
    message_id: str  # Gmail's internal message ID
    thread_id: str
    recipient_email: str
    recipient_name: str
    subject: str
    body: str
    sent_at: datetime
    replied: bool = False
    response_sent: bool = False
    last_history_id: Optional[str] = None
    references_chain: Optional[str] = None  # Store the full References chain
    rfc_message_id: Optional[str] = None  # Store the actual RFC-2822 Message-ID header
    user_id: Optional[str] = None  # New field
    order_id: Optional[str] = None  # New field
    record_date: Optional[datetime] = None  # New field - when processed
    cc_emails: Optional[str] = None # Comma-separated CC emails
    id: Optional[int] = None  # Auto-generated primary key

@dataclass
class EmailReply:
    """Represent a received reply"""
    message_id: str  # Gmail's internal message ID
    thread_id: str
    from_email: str
    subject: str
    body: str
    received_at: datetime
    history_id: str
    original_email: SentEmail # Represents the last known email state in DB for this thread before this reply
    rfc_message_id: Optional[str] = None  # Store the actual RFC-2822 Message-ID header
    # Fields to store headers of the incoming reply itself
    reply_to_header: Optional[str] = None
    reply_cc_header: Optional[str] = None

class PostgreSQLDatabaseManager:
    """PostgreSQL database operations manager"""
    
    def __init__(self):
        self.db_config = DB_CONFIG
        self.connection_pool = []
        self.pool_lock = threading.Lock()
        
        # Test connection and create tables if needed
        self._test_connection()
        self._create_tables()
        
        logger.info(f"PostgreSQL database manager initialized")
        logger.info(f"Connected to: {self.db_config['host']}:{self.db_config['port']}/{self.db_config['dbname']}")
    
    def _get_connection(self):
        """Get database connection from pool or create new one"""
        with self.pool_lock:
            if self.connection_pool:
                return self.connection_pool.pop()
            else:
                return psycopg2.connect(**self.db_config)
    
    def _return_connection(self, conn):
        """Return connection to pool"""
        with self.pool_lock:
            if len(self.connection_pool) < 10:  # Max 10 connections in pool
                self.connection_pool.append(conn)
            else:
                conn.close()
    
    def _test_connection(self):
        """Test database connection"""
        try:
            conn = psycopg2.connect(**self.db_config)
            cursor = conn.cursor()
            cursor.execute("SELECT 1")
            cursor.close()
            conn.close()
            logger.info("Database connection test successful")
        except Exception as e:
            logger.error(f"Database connection failed: {e}")
            raise
    
    def _create_tables(self):
        """Create required tables if they don't exist"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            # Create appointments_scheduling table
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS appointments_scheduling (
                    id SERIAL PRIMARY KEY,
                    user_id UUID NOT NULL,
                    schedule_datetime TIMESTAMP NOT NULL,
                    is_processed VARCHAR(1) DEFAULT 'N' CHECK (is_processed IN ('Y', 'N')),
                    email_id VARCHAR(255) NOT NULL,
                    order_id UUID,
                    po_ids VARCHAR(255) NOT NULL,
                    cc_email_id VARCHAR(255) NOT NULL,
                    phase INTEGER DEFAULT 1,
                    carrier_email_id VARCHAR(255),
                    carrier_cc_email_id VARCHAR(255),
                    is_reminder BOOLEAN DEFAULT FALSE,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );
            """)
            
            # Create order_activity table for appointment tracking
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS order_activity (
                    id SERIAL PRIMARY KEY,
                    order_id UUID UNIQUE NOT NULL,
                    is_appointment_confirmed BOOLEAN DEFAULT FALSE,
                    appointment_scheduled_at TIMESTAMP WITH TIME ZONE,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );
            """)
            
            # Create appointment_email_details table (SentEmail dataclass)
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS appointment_email_details (
                    id SERIAL PRIMARY KEY,
                    campaign_id VARCHAR(255),
                    message_id VARCHAR(255) UNIQUE NOT NULL,
                    thread_id VARCHAR(255) NOT NULL,
                    recipient_email VARCHAR(255) NOT NULL,
                    recipient_name VARCHAR(255),
                    subject TEXT,
                    body TEXT,
                    sent_at TIMESTAMP NOT NULL,
                    replied BOOLEAN DEFAULT FALSE,
                    response_sent BOOLEAN DEFAULT FALSE,
                    last_history_id VARCHAR(255),
                    references_chain TEXT,
                    rfc_message_id VARCHAR(255),
                    user_id UUID,
                    order_id VARCHAR(255),
                    record_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    cc_emails TEXT, 
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );
            """)
            
            # Create processed_replies table for duplicate prevention
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS processed_replies (
                    id SERIAL PRIMARY KEY,
                    message_id VARCHAR(255) UNIQUE NOT NULL,
                    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                );
            """)
            
            # Create indexes for better performance
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_appointments_scheduling_datetime 
                ON appointments_scheduling(schedule_datetime);
            """)
            
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_appointments_scheduling_processed 
                ON appointments_scheduling(is_processed);
            """)
            
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_appointment_email_details_thread 
                ON appointment_email_details(thread_id);
            """)
            
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_appointment_email_details_message 
                ON appointment_email_details(message_id);
            """)
            
            conn.commit()
            cursor.close()
            logger.info("Database tables created/verified successfully")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error creating tables: {e}")
            raise
        finally:
            self._return_connection(conn)
    
    def get_pending_appointments(self) -> List[AppointmentScheduling]:
        """Get appointments scheduled within the past hour that haven't been processed"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            # Get appointments from past hour that are not processed
            # Exclude phase 2 appointments (phase is null or not 2)
            # Exclude reminder appointments (is_reminder is not true)
            one_hour_ago = datetime.now() - timedelta(hours=1)
            current_time = datetime.now()
            
            cursor.execute("""
                SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
                FROM appointments_scheduling 
                WHERE schedule_datetime BETWEEN %s AND %s 
                AND is_processed = 'N'
                AND (phase IS NULL OR phase::integer != 2)
                AND (is_reminder IS NULL OR is_reminder != TRUE)
            """, (one_hour_ago, current_time))
            
            appointments = []
            for row in cursor.fetchall():
                appointments.append(AppointmentScheduling(
                    id=row['id'],
                    user_id=str(row['user_id']),
                    schedule_datetime=row['schedule_datetime'],
                    is_processed=row['is_processed'],
                    email_id=row['email_id'],
                    order_id=str(row['order_id']) if row['order_id'] else None,
                    po_ids=row['po_ids'],
                    cc_email_id=row['cc_email_id'],
                    phase=row['phase'],
                    carrier_email_id=row['carrier_email_id'],
                    carrier_cc_email_id=row['carrier_cc_email_id'],
                    is_reminder=row['is_reminder'],
                    address=row['address']
                ))
            
            cursor.close()
            logger.info(f"Found {len(appointments)} pending appointments")
            return appointments
            
        except Exception as e:
            logger.error(f"Error fetching pending appointments: {e}")
            return []
        finally:
            self._return_connection(conn)
    
    def get_phase2_appointments(self) -> List[AppointmentScheduling]:
        """Get phase 2 appointments that are not processed and ready for carrier emails"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            # Get phase 2 appointments that are not processed and have carrier email details
            one_hour_ago = datetime.now() - timedelta(hours=1)
            current_time = datetime.now()
            
            cursor.execute("""
                SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
                FROM appointments_scheduling 
                WHERE schedule_datetime BETWEEN %s AND %s 
                AND is_processed = 'N'
                AND phase::integer = 2
                AND carrier_email_id IS NOT NULL
                AND carrier_cc_email_id IS NOT NULL
            """, (one_hour_ago, current_time))
            
            appointments = []
            for row in cursor.fetchall():
                appointments.append(AppointmentScheduling(
                    id=row['id'],
                    user_id=str(row['user_id']),
                    schedule_datetime=row['schedule_datetime'],
                    is_processed=row['is_processed'],
                    email_id=row['email_id'],
                    order_id=str(row['order_id']) if row['order_id'] else None,
                    po_ids=row['po_ids'],
                    cc_email_id=row['cc_email_id'],
                    phase=row['phase'],
                    carrier_email_id=row['carrier_email_id'],
                    carrier_cc_email_id=row['carrier_cc_email_id'],
                    is_reminder=row['is_reminder']
                ))
            
            cursor.close()
            logger.info(f"Found {len(appointments)} phase 2 carrier appointments")
            return appointments
            
        except Exception as e:
            logger.error(f"Error fetching phase 2 appointments: {e}")
            return []
        finally:
            self._return_connection(conn)
    
    def get_reminder_appointments(self) -> List[AppointmentScheduling]:
        """Get reminder appointments with confirmed datetime from order_activity table"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            # Get reminder appointments with JOIN to order_activity
            one_hour_ago = datetime.now() - timedelta(hours=1)
            current_time = datetime.now()
            
            cursor.execute("""
                SELECT 
                    a.id, a.user_id, a.schedule_datetime, a.is_processed, a.email_id, 
                    a.order_id, a.po_ids, a.cc_email_id, a.phase, a.carrier_email_id, 
                    a.carrier_cc_email_id, a.is_reminder,
                    o.appointment_scheduled_at as confirmed_appointment_time
                FROM appointments_scheduling a
                INNER JOIN order_activity o ON a.order_id::uuid = o.order_id::uuid
                WHERE a.schedule_datetime BETWEEN %s AND %s 
                AND a.is_processed = 'N'
                AND a.is_reminder = TRUE
                AND a.carrier_email_id IS NOT NULL
                AND a.carrier_cc_email_id IS NOT NULL
                AND o.appointment_scheduled_at IS NOT NULL
            """, (one_hour_ago, current_time))
            
            appointments = []
            for row in cursor.fetchall():
                appointments.append(AppointmentScheduling(
                    id=row['id'],
                    user_id=str(row['user_id']),
                    schedule_datetime=row['schedule_datetime'],
                    is_processed=row['is_processed'],
                    email_id=row['email_id'],
                    order_id=str(row['order_id']) if row['order_id'] else None,
                    po_ids=row['po_ids'],
                    cc_email_id=row['cc_email_id'],
                    phase=row['phase'],
                    carrier_email_id=row['carrier_email_id'],
                    carrier_cc_email_id=row['carrier_cc_email_id'],
                    is_reminder=row['is_reminder'],
                    confirmed_appointment_time=row['confirmed_appointment_time']
                ))
            
            cursor.close()
            logger.info(f"Found {len(appointments)} reminder appointments")
            return appointments
            
        except Exception as e:
            logger.error(f"Error fetching reminder appointments: {e}")
            return []
        finally:
            self._return_connection(conn)
    
    def mark_appointment_processed(self, appointment_id: int, success: bool):
        """Mark appointment as processed (Y for success, N for failure)"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            status = 'Y' if success else 'N'
            cursor.execute("""
                UPDATE appointments_scheduling 
                SET is_processed = %s, updated_at = CURRENT_TIMESTAMP 
                WHERE id = %s
            """, (status, appointment_id))
            
            conn.commit()
            cursor.close()
            logger.info(f"Appointment {appointment_id} marked as processed: {status}")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error marking appointment as processed: {e}")
        finally:
            self._return_connection(conn)
    
    def mark_phase2_appointment_processed(self, appointment_id: int, success: bool):
        """Mark a phase 2 appointment as processed"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            cursor.execute("""
                UPDATE appointments_scheduling 
                SET is_processed = 'Y',  -- Mark as processed
                    updated_at = CURRENT_TIMESTAMP
                WHERE id = %s
            """, (appointment_id,))
            conn.commit()
            logger.info(f"Marked phase 2 appointment {appointment_id} as processed")
        except Exception as e:
            logger.error(f"Error marking phase 2 appointment as processed: {e}")
            conn.rollback()
        finally:
            self._return_connection(conn)
    
    def mark_reminder_appointment_processed(self, appointment_id: int, success: bool):
        """Mark reminder appointment as processed"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            # Mark as processed (Y for success, N for failure) and update timestamp
            status = 'Y' if success else 'N'
            cursor.execute("""
                UPDATE appointments_scheduling 
                SET is_processed = %s, updated_at = CURRENT_TIMESTAMP 
                WHERE id = %s
            """, (status, appointment_id))
            
            conn.commit()
            cursor.close()
            logger.info(f"Reminder appointment {appointment_id} marked as processed: {status}")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error marking reminder appointment as processed: {e}")
        finally:
            self._return_connection(conn)
    
    def store_sent_email(self, sent_email: SentEmail):
        """Store sent email in appointment_email_details table"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            # Set record_date to current time if not provided
            if sent_email.record_date is None:
                sent_email.record_date = datetime.now()
            
            cursor.execute("""
                INSERT INTO appointment_email_details (
                    campaign_id, message_id, thread_id, recipient_email, recipient_name,
                    subject, body, sent_at, replied, response_sent, last_history_id,
                    references_chain, rfc_message_id, user_id, order_id, record_date, cc_emails
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                )
            """, (
                sent_email.campaign_id, sent_email.message_id, sent_email.thread_id,
                sent_email.recipient_email, sent_email.recipient_name, sent_email.subject,
                sent_email.body, sent_email.sent_at, sent_email.replied, sent_email.response_sent,
                sent_email.last_history_id, sent_email.references_chain, sent_email.rfc_message_id,
                sent_email.user_id, sent_email.order_id, sent_email.record_date, sent_email.cc_emails
            ))
            
            conn.commit()
            cursor.close()
            logger.info(f"Stored sent email: {sent_email.message_id}")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error storing sent email: {e}")
        finally:
            self._return_connection(conn)
    
    def get_sent_email_by_thread(self, thread_id: str) -> Optional[SentEmail]:
        """Get sent email by thread ID from appointment_email_details table"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            cursor.execute("""
                SELECT * FROM appointment_email_details 
                WHERE thread_id = %s 
                ORDER BY sent_at DESC 
                LIMIT 1
            """, (thread_id,))
            
            row = cursor.fetchone()
            cursor.close()
            
            if row:
                return SentEmail(
                    id=row['id'],
                    campaign_id=row['campaign_id'],
                    message_id=row['message_id'],
                    thread_id=row['thread_id'],
                    recipient_email=row['recipient_email'],
                    recipient_name=row['recipient_name'],
                    subject=row['subject'],
                    body=row['body'],
                    sent_at=row['sent_at'],
                    replied=row['replied'],
                    response_sent=row['response_sent'],
                    last_history_id=row['last_history_id'],
                    references_chain=row['references_chain'],
                    rfc_message_id=row['rfc_message_id'],
                    user_id=str(row['user_id']) if row['user_id'] else None,
                    order_id=row['order_id'],
                    record_date=row['record_date'],
                    cc_emails=row['cc_emails']
                )
            
            logger.info(f"No sent email found for thread {thread_id}")
            return None
            
        except Exception as e:
            logger.error(f"Error fetching sent email by thread: {e}")
            return None
        finally:
            self._return_connection(conn)
    
    def mark_reply_processed(self, message_id: str):
        """Mark reply as processed to prevent duplicates"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            cursor.execute("""
                INSERT INTO processed_replies (message_id) 
                VALUES (%s) 
                ON CONFLICT (message_id) DO NOTHING
            """, (message_id,))
            
            conn.commit()
            cursor.close()
            logger.info(f"Reply {message_id} marked as processed")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error marking reply as processed: {e}")
        finally:
            self._return_connection(conn)
    
    def is_reply_processed(self, message_id: str) -> bool:
        """Check if reply has been processed"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            cursor.execute("""
                SELECT 1 FROM processed_replies WHERE message_id = %s
            """, (message_id,))
            
            result = cursor.fetchone()
            cursor.close()
            
            return result is not None
            
        except Exception as e:
            logger.error(f"Error checking if reply is processed: {e}")
            return False
        finally:
            self._return_connection(conn)
    
    def update_sent_email_references(self, message_id: str, references_chain: str):
        """Update references chain for sent email"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            cursor.execute("""
                UPDATE appointment_email_details 
                SET references_chain = %s, updated_at = CURRENT_TIMESTAMP 
                WHERE message_id = %s
            """, (references_chain, message_id))
            
            conn.commit()
            cursor.close()
            logger.info(f"Updated references chain for message {message_id}")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error updating references chain: {e}")
        finally:
            self._return_connection(conn)
    
    def get_thread_emails(self, thread_id: str) -> List[SentEmail]:
        """Get all emails in a thread for context"""
        conn = self._get_connection()
        try:
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            cursor.execute("""
                SELECT * FROM appointment_email_details 
                WHERE thread_id = %s 
                ORDER BY sent_at ASC
            """, (thread_id,))
            
            emails = []
            for row in cursor.fetchall():
                emails.append(SentEmail(
                    id=row['id'],
                    campaign_id=row['campaign_id'],
                    message_id=row['message_id'],
                    thread_id=row['thread_id'],
                    recipient_email=row['recipient_email'],
                    recipient_name=row['recipient_name'],
                    subject=row['subject'],
                    body=row['body'],
                    sent_at=row['sent_at'],
                    replied=row['replied'],
                    response_sent=row['response_sent'],
                    last_history_id=row['last_history_id'],
                    references_chain=row['references_chain'],
                    rfc_message_id=row['rfc_message_id'],
                    user_id=str(row['user_id']) if row['user_id'] else None,
                    order_id=row['order_id'],
                    record_date=row['record_date'],
                    cc_emails=row['cc_emails']
                ))
            
            cursor.close()
            logger.info(f"Found {len(emails)} emails in thread {thread_id}")
            return emails
            
        except Exception as e:
            logger.error(f"Error fetching thread emails: {e}")
            return []
        finally:
            self._return_connection(conn)
    
    def update_order_activity(self, order_id: str, is_confirmed: bool, appointment_time: Optional[datetime] = None):
        """Update order_activity table with appointment confirmation status"""
        if not order_id:
            logger.warning("No order_id provided for order_activity update")
            return
        
        conn = self._get_connection()
        try:
            cursor = conn.cursor()
            
            # Validate order_id is proper UUID
            order_id_uuid = self._validate_and_convert_uuid(order_id)
            if not order_id_uuid:
                logger.error(f"Invalid order_id UUID: {order_id}")
                return
            
            # Ensure boolean type
            is_confirmed_bool = bool(is_confirmed)
            
            cursor.execute("""
                INSERT INTO order_activity (order_id, is_appointment_confirmed, appointment_scheduled_at)
                VALUES (%s, %s, %s)
                ON CONFLICT (order_id) 
                DO UPDATE SET 
                    is_appointment_confirmed = EXCLUDED.is_appointment_confirmed,
                    appointment_scheduled_at = EXCLUDED.appointment_scheduled_at,
                    updated_at = CURRENT_TIMESTAMP
            """, (order_id_uuid, is_confirmed_bool, appointment_time))
            
            conn.commit()
            cursor.close()
            logger.info(f"Updated order_activity for order {order_id_uuid}: confirmed={is_confirmed_bool}, time={appointment_time}")
            
        except Exception as e:
            conn.rollback()
            logger.error(f"Error updating order_activity: {e}")
        finally:
            self._return_connection(conn)
    
    def _validate_and_convert_uuid(self, uuid_string: str) -> str:
        """Validate and convert UUID string"""
        try:
            # This will raise ValueError if invalid UUID
            import uuid
            uuid_obj = uuid.UUID(uuid_string)
            return str(uuid_obj)
        except (ValueError, TypeError):
            return None
 
class GmailPushHandler:
    """Handles Gmail push notifications via Cloud Pub/Sub"""
    
    def __init__(self, gmail_service, database_manager, ai_responder):
        self.gmail_service = gmail_service
        self.database_manager = database_manager
        self.ai_responder = ai_responder
        self.processing_lock = threading.Lock()
        
        # Store last processed history ID to avoid duplicates
        self.last_history_id = None
        
        logger.info("Gmail push handler initialized")
    
    def setup_push_notifications(self):
        """Set up Gmail push notifications"""
        try:
            # Get Google Cloud project details from environment
            project_id = os.getenv('GOOGLE_CLOUD_PROJECT_ID')
            topic_name = os.getenv('PUBSUB_TOPIC_NAME', 'gmail-notifications')
            
            if not project_id:
                logger.error("GOOGLE_CLOUD_PROJECT_ID not set in environment")
                return False
            
            topic_path = f"projects/{project_id}/topics/{topic_name}"
            
            # Set up Gmail watch request
            watch_request = {
                'topicName': topic_path,
                'labelIds': ['INBOX'],
                'labelFilterBehavior': 'include'
            }
            
            result = self.gmail_service.users().watch(userId='me', body=watch_request).execute()
            
            self.last_history_id = result.get('historyId')
            logger.info(f"Gmail push notifications set up. History ID: {self.last_history_id}")
            
            return True
            
        except Exception as e:
            logger.error(f"Failed to set up push notifications: {e}")
            return False
    
    def handle_push_notification(self, notification_data: dict):
        """Handle incoming push notification from Pub/Sub"""
        with self.processing_lock:
            try:
                # Decode the notification data
                if 'message' in notification_data and 'data' in notification_data['message']:
                    encoded_data = notification_data['message']['data']
                    decoded_data = base64.b64decode(encoded_data).decode('utf-8')
                    gmail_notification = json.loads(decoded_data)
                    
                    email_address = gmail_notification.get('emailAddress')
                    history_id = gmail_notification.get('historyId')
                    
                    logger.info(f"Push notification received for {email_address}, history ID: {history_id}")
                    
                    # Process the notification
                    self.process_history_changes(history_id)
                    
                else:
                    logger.warning("Invalid notification format received")
                    
            except Exception as e:
                logger.error(f"Error handling push notification: {e}")
    
    def process_history_changes(self, current_history_id: str):
        """Process Gmail history changes"""
        try:
            if not self.last_history_id:
                logger.info("No previous history ID, skipping history processing")
                self.last_history_id = current_history_id
                return
            
            logger.info(f"Processing history from {self.last_history_id} to {current_history_id}")
            
            # Get history changes since last known history ID
            history_request = self.gmail_service.users().history().list(
                userId='me',
                startHistoryId=self.last_history_id,
                historyTypes=['messageAdded'],
                labelId='INBOX'
            )
            
            history_response = history_request.execute()
            history_records = history_response.get('history', [])
            
            logger.info(f"Processing {len(history_records)} history records")
            
            for record in history_records:
                messages_added = record.get('messagesAdded', [])
                logger.info(f"History record has {len(messages_added)} messages added")
                
                for message_added in messages_added:
                    message = message_added.get('message', {})
                    message_id = message.get('id')
                    thread_id = message.get('threadId')
                    
                    logger.info(f"Processing message {message_id} in thread {thread_id}")
                    
                    if message_id and thread_id:
                        self.process_new_message(message_id, thread_id)
            
            # Update last processed history ID
            self.last_history_id = current_history_id
            logger.info(f"Updated last history ID to: {current_history_id}")
            
        except Exception as e:
            logger.error(f"Error processing history changes: {e}")
    
    def process_new_message(self, message_id: str, thread_id: str):
        """Process a new message to check if it's a reply"""
        try:
            # Check if this specific message has already been processed
            if self.database_manager.is_reply_processed(message_id):
                logger.info(f"Message {message_id} already processed, skipping")
                return
            
            # Get the original sent email for this thread
            sent_email = self.database_manager.get_sent_email_by_thread(thread_id)
            if not sent_email:
                logger.info(f"No sent email found for thread {thread_id}, skipping")
                return
            
            # Check if this is a phase 2 or reminder email (skip AI response)
            if sent_email.campaign_id.startswith(('phase2_', 'reminder_')):
                logger.info(f"Skipping AI response for {sent_email.campaign_id} email")
                # Still mark as processed to prevent re-processing
                self.database_manager.mark_reply_processed(message_id)
                return
            
            # Get the message details
            message = self.gmail_service.users().messages().get(
                userId='me',
                id=message_id,
                format='full'
            ).execute()
            
            # Extract headers
            headers = {h['name'].lower(): h['value'] for h in message['payload'].get('headers', [])}
            from_email_header = headers.get('from', '')
            subject = headers.get('subject', '')
            rfc_message_id = headers.get('message-id', '')
            incoming_to_header = headers.get('to', '')
            incoming_cc_header = headers.get('cc', '')
            incoming_references_header = headers.get('references', '')
            incoming_in_reply_to_header = headers.get('in-reply-to', '')

            sender_email = self._extract_email_address(from_email_header)
            
            # Get our system's sending email address
            system_sender_email = os.getenv('SENDER_EMAIL')
            if not system_sender_email:
                logger.error("SENDER_EMAIL environment variable not set")
                return
            
            if system_sender_email and sender_email == self._extract_email_address(system_sender_email):
                logger.info(f"Message {message_id} is from our own system ({sender_email}), skipping to prevent loop.")
                return

            logger.info(f"Extracted sender: '{sender_email}'")
            logger.info(f"Incoming To header: '{incoming_to_header}'")
            logger.info(f"Incoming Cc header: '{incoming_cc_header}'")
            
            # Extract message body
            body = self.extract_message_body(message)
            
            # Create reply object
            reply = EmailReply(
                message_id=message_id,
                thread_id=thread_id,
                from_email=from_email_header,
                subject=subject,
                body=body,
                received_at=datetime.now(),
                history_id=self.last_history_id,
                original_email=sent_email,
                rfc_message_id=rfc_message_id,
                reply_to_header=incoming_to_header,
                reply_cc_header=incoming_cc_header
            )
            
            logger.info(f"Reply detected from {sender_email} in thread {thread_id}")
            
            # Store the incoming reply as part of thread context
            reply_as_sent_email_record = SentEmail(
                campaign_id=f"reply_{sent_email.campaign_id or 'unknown'}",
                message_id=message_id,
                thread_id=thread_id,
                recipient_email=sender_email,
                recipient_name=self._extract_email_address(from_email_header),
                subject=subject,
                body=body,
                sent_at=datetime.now(),
                replied=False,
                response_sent=False,
                last_history_id=None,
                references_chain=incoming_references_header if incoming_references_header else (incoming_in_reply_to_header if incoming_in_reply_to_header else None),
                rfc_message_id=rfc_message_id,
                user_id=sent_email.user_id,
                order_id=sent_email.order_id,
                record_date=datetime.now(),
                cc_emails=incoming_cc_header
            )
            
            self.database_manager.store_sent_email(reply_as_sent_email_record)
            
            # Generate and send AI response only for original flow
            self.ai_responder.process_reply(reply)
            
            # Mark this specific message as processed
            self.database_manager.mark_reply_processed(message_id)
            
        except Exception as e:
            logger.error(f"Error processing message {message_id}: {e}")
    
    def extract_message_body(self, message):
        """Extract plain text body from Gmail message"""
        try:
            payload = message.get('payload', {})
            
            # Handle multipart messages
            if payload.get('mimeType') == 'multipart/alternative':
                parts = payload.get('parts', [])
                for part in parts:
                    if part.get('mimeType') == 'text/plain':
                        data = part.get('body', {}).get('data', '')
                        if data:
                            return base64.urlsafe_b64decode(data).decode('utf-8')
            
            # Handle simple text messages
            elif payload.get('mimeType') == 'text/plain':
                data = payload.get('body', {}).get('data', '')
                if data:
                    return base64.urlsafe_b64decode(data).decode('utf-8')
            
            # Handle nested multipart
            elif payload.get('mimeType', '').startswith('multipart/'):
                return self._extract_from_multipart(payload)
            
            return "Unable to extract message body"
            
        except Exception as e:
            logger.error(f"Failed to extract message body: {e}")
            return "Error extracting message body"
    
    def _extract_from_multipart(self, payload):
        """Extract text from multipart payload"""
        parts = payload.get('parts', [])
        for part in parts:
            if part.get('mimeType') == 'text/plain':
                data = part.get('body', {}).get('data', '')
                if data:
                    return base64.urlsafe_b64decode(data).decode('utf-8')
            elif part.get('mimeType', '').startswith('multipart/'):
                # Nested multipart
                nested_parts = part.get('parts', [])
                for nested_part in nested_parts:
                    if nested_part.get('mimeType') == 'text/plain':
                        data = nested_part.get('body', {}).get('data', '')
                        if data:
                            return base64.urlsafe_b64decode(data).decode('utf-8')
        return "Unable to extract message body"

    def _extract_email_address(self, email_string):
        """Extract email address from 'Name <email>' or just 'email' format"""
        if '<' in email_string and '>' in email_string:
            # Extract email from "Name <email@domain.com>" format
            match = re.search(r'<(.+?)>', email_string)
            return match.group(1).lower() if match else email_string.lower()
        return email_string.lower().strip()

class AIResponseGenerator:
    """Generates AI responses using Gemini 1.5 Flash"""
    
    def __init__(self, gmail_service, database_manager):
        self.gmail_service = gmail_service
        self.database_manager = database_manager
        
        # Initialize Gemini
        self.gemini_api_key = os.getenv('GEMINI_API_KEY')
        if self.gemini_api_key:
            genai.configure(api_key=self.gemini_api_key)
            self.gemini_model = genai.GenerativeModel('gemini-1.5-flash')
            logger.info("Gemini 1.5 Flash initialized")
        else:
            logger.error("GEMINI_API_KEY not found in environment variables")
            self.gemini_model = None
        
        # Business configuration
        self.business_name = os.getenv('BUSINESS_NAME', 'Your Business')
        self.business_hours = f"{os.getenv('BUSINESS_HOURS_START', '09:00')} - {os.getenv('BUSINESS_HOURS_END', '17:00')}"
        
        # Response processing lock to prevent duplicates
        self.response_lock = threading.Lock()
    
    def process_reply(self, reply: EmailReply):
        """Process a reply and send AI response"""
        with self.response_lock:
            try:
                logger.info(f"Processing reply from {reply.from_email} in thread {reply.thread_id}")
                
                # Generate AI response
                ai_response = self.generate_ai_response(reply)
                
                # Send response
                success = self.send_ai_response(reply, ai_response)
                
                if success:
                    logger.info(f"AI response sent successfully to {reply.from_email}")
                    logger.info(f"Continuous conversation enabled for thread {reply.thread_id}")
                else:
                    logger.error(f"Failed to send AI response to {reply.from_email}")
                
            except Exception as e:
                logger.error(f"Error processing reply: {e}")
    
    def generate_ai_response(self, reply: EmailReply) -> str:
        """Generate AI response using Gemini 1.5 Flash with full thread context"""
        if not self.gemini_model:
            return self._get_fallback_response(reply)
        
        try:
            # Get full thread context
            thread_emails = self.database_manager.get_thread_emails(reply.thread_id)
            
            # Build conversation history
            conversation_history = ""
            appointment_info = None
            
            for i, email in enumerate(thread_emails):
                conversation_history += f"\n--- Email {i+1} ({email.sent_at.strftime('%Y-%m-%d %H:%M')}) ---\n"
                conversation_history += f"Subject: {email.subject}\n"
                conversation_history += f"Body: {email.body[:500]}{'...' if len(email.body) > 500 else ''}\n"
                
                # Extract appointment information
                appointment_info = self._extract_appointment_info(email.body, appointment_info)
            
            # Add the current reply
            conversation_history += f"\n--- Latest Reply ({reply.received_at.strftime('%Y-%m-%d %H:%M')}) ---\n"
            conversation_history += f"From: {reply.from_email}\n"
            conversation_history += f"Subject: {reply.subject}\n"
            conversation_history += f"Body: {reply.body[:500]}{'...' if len(reply.body) > 500 else ''}\n"
            
            # Extract appointment info from current reply
            appointment_info = self._extract_appointment_info(reply.body, appointment_info)
            
            appointment_context = ""
            if appointment_info:
                appointment_context = f"\nAPPOINTMENT SCHEDULED: {appointment_info}\n"

            prompt = f"""
You are an AI assistant for {self.business_name}, a B2B logistics SaaS platform coordinating with courier partners.

CONTEXT:
- Business Hours: {self.business_hours}
- This is an ongoing logistics coordination thread for {self.business_name}.
- The goal is to quickly confirm a shipping slot.

FULL CONVERSATION HISTORY (most recent messages are most relevant):
{conversation_history}
{appointment_context}

INSTRUCTIONS:
1. Review the conversation history for context, focusing on the latest messages.
2. Generate a VERY CONCISE response (1 short paragraph, ideally 1-2 sentences, MAX 3 unless absolutely necessary for clarity).
3. If the user proposes a specific date/time for a slot, ACCEPT IT directly and confirm. Do NOT try to negotiate for a different or closer slot.
4. If the user asks a direct question, answer it concisely.
5. If the user provides information, acknowledge it briefly.
6. Maintain a professional B2B tone.
7. Your primary goal is to confirm the slot or get information needed to confirm it, with minimal back-and-forth.

EXAMPLES of concise responses:
- User: "We have a slot available on June 1st at 2 PM." AI: "Confirmed for June 1st at 2 PM. Please send booking details."
- User: "What is the weight of the shipment?" AI: "The shipment weight is 500kg."
- User: "Can you confirm pickup address?" AI: "Yes, the pickup address is 123 Main St."

Generate ONLY the email response content (no subject line, no headers):
"""
            
            response = self.gemini_model.generate_content(prompt)
            ai_response = response.text.strip()
            
            # Check if appointment is confirmed using LLM
            is_confirmed, confirmed_datetime = self._check_appointment_confirmation_with_llm(conversation_history)
            
            # Update order_activity if appointment is confirmed and we have order_id
            if is_confirmed and reply.original_email.user_id:
                # Get order_id from the original appointment
                original_appointment = self._get_appointment_by_user_id(reply.original_email.user_id)
                if original_appointment and original_appointment.order_id:
                    self.database_manager.update_order_activity(
                        original_appointment.order_id, 
                        is_confirmed, 
                        confirmed_datetime
                    )
            
            logger.info(f"Generated {len(ai_response)} character response with full thread context")
            if appointment_info:
                logger.info(f"Appointment info extracted: {appointment_info}")
            if is_confirmed:
                logger.info(f"Appointment CONFIRMED: {confirmed_datetime}")
            
            return ai_response
            
        except Exception as e:
            logger.error(f"Gemini AI failed: {e}")
            return self._get_fallback_response(reply)
    
    def _extract_appointment_info(self, text: str, existing_info: str = None) -> str:
        """Extract appointment date/time information from email text"""
        import re
        
        # Common appointment patterns
        patterns = [
            r'(?:appointment|meeting|slot|pickup|delivery).*?(?:on|at|for)\s+([A-Za-z]+\s+\d{1,2}(?:st|nd|rd|th)?,?\s+\d{4}(?:\s+at\s+\d{1,2}:\d{2}(?:\s*[AP]M)?)?)',
            r'(?:scheduled|confirmed|booked).*?(?:for|on|at)\s+([A-Za-z]+\s+\d{1,2}(?:st|nd|rd|th)?,?\s+\d{4}(?:\s+at\s+\d{1,2}:\d{2}(?:\s*[AP]M)?)?)',
            r'(\d{1,2}[/-]\d{1,2}[/-]\d{4}(?:\s+at\s+\d{1,2}:\d{2}(?:\s*[AP]M)?)?)',
            r'(\d{4}-\d{2}-\d{2}(?:\s+\d{2}:\d{2})?)',
        ]
        
        for pattern in patterns:
            matches = re.findall(pattern, text, re.IGNORECASE)
            if matches:
                new_info = matches[0].strip()
                if existing_info and new_info not in existing_info:
                    return f"{existing_info}, {new_info}"
                elif not existing_info:
                    return new_info
        
        return existing_info
    
    def _check_appointment_confirmation_with_llm(self, conversation_history: str) -> tuple[bool, Optional[datetime]]:
        """Use LLM to detect if appointment is confirmed and extract datetime"""
        if not self.gemini_model:
            return False, None
        
        try:
            prompt = f"""
Analyze this email conversation to determine if an appointment has been CONFIRMED/SCHEDULED.

CONVERSATION:
{conversation_history}

TASK:
1. Determine if an appointment is CONFIRMED (not just proposed or discussed)
2. If confirmed, extract the exact date and time

CONFIRMATION INDICATORS:
- Words like "confirmed", "scheduled", "booked", "agreed", "set for"
- Specific date/time mentioned with confirmation language
- Mutual agreement on specific timing

RESPONSE FORMAT (JSON only):
{{
    "is_confirmed": true/false,
    "appointment_datetime": "YYYY-MM-DD HH:MM" or null,
    "confidence": "high/medium/low"
}}

Respond with ONLY the JSON, no other text:
"""
            
            response = self.gemini_model.generate_content(prompt)
            result_text = response.text.strip()
            
            # Clean up response to extract JSON
            if result_text.startswith('```json'):
                result_text = result_text[7:-3]
            elif result_text.startswith('```'):
                result_text = result_text[3:-3]
            
            import json
            result = json.loads(result_text)
            
            is_confirmed = bool(result.get('is_confirmed', False))
            appointment_str = result.get('appointment_datetime')
            
            appointment_datetime = None
            if is_confirmed and appointment_str:
                try:
                    from datetime import datetime
                    appointment_datetime = datetime.strptime(appointment_str, '%Y-%m-%d %H:%M')
                except ValueError:
                    try:
                        appointment_datetime = datetime.strptime(appointment_str, '%Y-%m-%d')
                    except ValueError:
                        logger.warning(f"Could not parse appointment datetime: {appointment_str}")
            
            logger.info(f"LLM appointment analysis: confirmed={is_confirmed}, datetime={appointment_datetime}")
            return is_confirmed, appointment_datetime
            
        except Exception as e:
            logger.error(f"LLM appointment confirmation check failed: {e}")
            return False, None
    
    def _get_appointment_by_user_id(self, user_id: str) -> Optional[AppointmentScheduling]:
        """Get appointment details by user_id"""
        try:
            conn = self.database_manager._get_connection()
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            cursor.execute("""
                SELECT id, user_id, schedule_datetime, is_processed, email_id, order_id, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder
                FROM appointments_scheduling 
                WHERE user_id = %s 
                ORDER BY created_at DESC 
                LIMIT 1
            """, (user_id,))
            
            row = cursor.fetchone()
            cursor.close()
            self.database_manager._return_connection(conn)
            
            if row:
                return AppointmentScheduling(
                    id=row['id'],
                    user_id=str(row['user_id']),
                    schedule_datetime=row['schedule_datetime'],
                    is_processed=row['is_processed'],
                    email_id=row['email_id'],
                    order_id=str(row['order_id']) if row['order_id'] else None,
                    po_ids=row['po_ids'],
                    cc_email_id=row['cc_email_id'],
                    phase=row['phase'],
                    carrier_email_id=row['carrier_email_id'],
                    carrier_cc_email_id=row['carrier_cc_email_id'],
                    is_reminder=row['is_reminder'],
                    address=row['customer_address']
                )
            
            return None
            
        except Exception as e:
            logger.error(f"Error getting appointment by user_id: {e}")
            return None
    
    def _get_fallback_response(self, reply: EmailReply) -> str:
        """Fallback response if AI fails"""
        # Try to determine if the reply seems to offer a slot
        if "slot" in reply.body.lower() or "available" in reply.body.lower() or re.search(r'\d{1,2}[/-]\d{1,2}', reply.body):
            return f"Thank you for the update on slot availability for {reply.original_email.recipient_name}. Please provide the specific date and time so we can confirm. \n\nBest,\n{self.business_name}"
        
        return f"""Thank you for your message, {reply.original_email.recipient_name}.
We are awaiting slot confirmation details.
Best regards,
{self.business_name}"""
    
    def send_ai_response(self, reply: EmailReply, ai_response: str) -> bool:
        """Send AI-generated response using proper RFC-2822 threading"""
        try:
            # CRITICAL: Add "Re:" prefix to subject for proper threading
            subject = f"Re: {reply.original_email.subject}" if not reply.subject.startswith("Re:") else reply.subject
            
            # Create message using EmailMessage (Google's recommended approach)
            message = EmailMessage()
            message.set_content(ai_response)
            
            # The AI should reply TO the sender of the incoming message
            message['To'] = self._extract_email_address(reply.from_email) 
            message['From'] = os.getenv('SENDER_EMAIL', 'noreply@yourbusiness.com')
            message['Subject'] = subject
            
            # --- Build CC list for Reply-to-All behavior ---
            system_email = os.getenv('SENDER_EMAIL', 'noreply@yourbusiness.com').lower()
            # ai_reply_to_address is now message['To']
            actual_ai_reply_to_address = self._extract_email_address(message['To']).lower()
            
            cc_list = set() # Use a set to avoid duplicates
            
            # Add emails from the incoming message's TO header (reply.reply_to_header)
            if reply.reply_to_header:
                to_emails_on_incoming = [self._extract_email_address(e) for e in reply.reply_to_header.split(',')]
                for email in to_emails_on_incoming:
                    if email and email != system_email and email != actual_ai_reply_to_address:
                        cc_list.add(email)
            
            # Add emails from the incoming message's CC header (reply.reply_cc_header)
            if reply.reply_cc_header:
                cc_emails_on_incoming = [self._extract_email_address(e) for e in reply.reply_cc_header.split(',')]
                for email in cc_emails_on_incoming:
                    if email and email != system_email and email != actual_ai_reply_to_address:
                        cc_list.add(email)
            
            actual_cc_string = None
            if cc_list:
                actual_cc_string = ",".join(sorted(list(cc_list)))
                message['Cc'] = actual_cc_string
            # --- End of CC list building ---
            
            # CRUCIAL: Use actual RFC-2822 Message-IDs for proper threading
            if not reply.rfc_message_id:
                logger.error(f"No RFC Message-ID found for reply {reply.message_id}")
                return False
            
            # In-Reply-To must reference the actual RFC Message-ID of the message we're replying to
            message['In-Reply-To'] = reply.rfc_message_id
            
            # References header must include ALL previous RFC Message-IDs in the thread
            if reply.original_email.references_chain:
                # Add the reply's RFC Message-ID to the existing chain
                new_references_chain = f"{reply.original_email.references_chain} {reply.rfc_message_id}"
            else:
                # First reply - start with original email's RFC Message-ID and this reply's RFC Message-ID
                if reply.original_email.rfc_message_id:
                    new_references_chain = f"{reply.original_email.rfc_message_id} {reply.rfc_message_id}"
                else:
                    # Fallback if original email doesn't have RFC Message-ID
                    new_references_chain = reply.rfc_message_id
            
            message['References'] = new_references_chain
            
            # Generate proper Message-ID with angle brackets and proper domain
            sender_domain = os.getenv('SENDER_EMAIL', 'noreply@yourbusiness.com').split('@')[1]
            unique_id = f"{int(time.time())}.{abs(hash(reply.thread_id + reply.message_id)) % 99999}"
            ai_message_id = f"<{unique_id}@{sender_domain}>"
            message['Message-ID'] = ai_message_id
            
            # Encode message (Google's official way)
            encoded_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
            
            # CRITICAL: Send using the exact thread ID from Gmail with threadId specified
            send_message = {
                'raw': encoded_message,
                'threadId': reply.thread_id  # This must match the original thread
            }

            logger.info(f"Sending AI response to thread {reply.thread_id}")
            logger.info(f"Using RFC Message-IDs - Original: {reply.original_email.rfc_message_id}, Reply: {reply.rfc_message_id}")
            
            result = self.gmail_service.users().messages().send(
                userId='me',
                body=send_message
            ).execute()
            
            # Update the references chain with our AI response RFC Message-ID for future replies
            final_references_chain = f"{new_references_chain} {ai_message_id}"
            reply.original_email.references_chain = final_references_chain
            
            # Store the updated email details in database
            self.database_manager.store_sent_email(reply.original_email)
            
            # Also store this AI response as a new email detail record
            ai_email_details = SentEmail(
                campaign_id=f"ai_response_{reply.original_email.campaign_id or 'unknown'}",
                message_id=result['id'],
                thread_id=reply.thread_id,
                recipient_email=message['To'], # Actual To address of AI's email
                recipient_name=message['To'], # For simplicity, or parse name if available
                subject=subject,
                body=ai_response,
                sent_at=datetime.now(),
                references_chain=final_references_chain,
                rfc_message_id=ai_message_id,
                record_date=datetime.now(),
                cc_emails=actual_cc_string, # Actual CC string used by AI
                user_id=reply.original_email.user_id, # Carry over user_id
                order_id=reply.original_email.order_id # Carry over order_id
            )
            
            self.database_manager.store_sent_email(ai_email_details)
            
            # Verify the response is in the same thread
            response_thread_id = result.get('threadId')
            if response_thread_id != reply.thread_id:
                logger.warning(f"Threading mismatch! Expected: {reply.thread_id}, Got: {response_thread_id}")
            else:
                logger.info(f"AI response sent in correct thread: {reply.thread_id}")
            
            logger.info(f"AI response sent: {result['id']} in thread {reply.thread_id}")
            logger.info(f"AI To: {message['To']}")
            if message['Cc']:
                logger.info(f"AI Cc: {message['Cc']}")
            logger.info(f"RFC Threading headers - In-Reply-To: {reply.rfc_message_id}, References: {new_references_chain}")
            logger.info(f"Updated references chain: {final_references_chain}")
            logger.info(f"Subject with Re: prefix: {subject}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to send AI response: {e}")
            logger.error(f"Full error: {traceback.format_exc()}")
            return False

    def _extract_email_address(self, email_string: str) -> str:
        """Helper to extract a single email address from a string that might be 'Name <email>'"""
        if not email_string: return ''
        match = re.search(r'<(.+?)>', email_string)
        if match:
            return match.group(1).lower().strip()
        return email_string.lower().strip()

class EmailCronSender:
    """Handles sending emails based on cron schedule"""
    
    def __init__(self, gmail_service, database_manager):
        self.gmail_service = gmail_service
        self.database_manager = database_manager
        self.sender_email = os.getenv('SENDER_EMAIL')
        if not self.sender_email:
            raise ValueError("SENDER_EMAIL environment variable not set")
        
        logger.info(f"Logistics coordination sender initialized for: {self.sender_email}")

    def run_cron_job(self):
        """Main cron job execution - handle both regular logistics and phase 2 carrier emails"""
        logger.info("Starting cron job execution")
        
        try:
            # Process regular logistics requests (is_processed = 'N')
            pending_appointments = self.database_manager.get_pending_appointments()
            
            if pending_appointments:
                logger.info(f"Found {len(pending_appointments)} pending logistics requests")
                
                # Send logistics coordination emails for each request
                for appointment in pending_appointments:
                    success = self.send_logistics_coordination_email(appointment)
                    
                    # Mark logistics request as processed
                    self.database_manager.mark_appointment_processed(appointment.id, success)
                    
                    if success:
                        logger.info(f"Logistics request {appointment.id} sent successfully")
                    else:
                        logger.error(f"Logistics request {appointment.id} failed to send")
                    
                    # Small delay between emails
                    time.sleep(1)
            else:
                logger.info("No pending logistics requests found")
            
            # Process phase 2 carrier emails (is_processed = 'Y', phase = 2)
            phase2_appointments = self.database_manager.get_phase2_appointments()
            
            if phase2_appointments:
                logger.info(f"Found {len(phase2_appointments)} phase 2 carrier appointments")
                
                # Send carrier emails for each phase 2 request
                for appointment in phase2_appointments:
                    success = self.send_carrier_email(appointment)
                    
                    # Mark phase 2 appointment as processed
                    self.database_manager.mark_phase2_appointment_processed(appointment.id, success)
                    
                    if success:
                        logger.info(f"Phase 2 carrier email {appointment.id} sent successfully")
                    else:
                        logger.error(f"Phase 2 carrier email {appointment.id} failed to send")
                    
                    # Small delay between emails
                    time.sleep(1)
            else:
                logger.info("No phase 2 carrier appointments found")
            
            # Process reminder emails (is_reminder = TRUE)
            reminder_appointments = self.database_manager.get_reminder_appointments()
            
            if reminder_appointments:
                logger.info(f"Found {len(reminder_appointments)} reminder appointments")
                
                # Send reminder emails for each appointment
                for appointment in reminder_appointments:
                    success = self.send_reminder_email(appointment)
                    
                    # Mark reminder appointment as processed
                    self.database_manager.mark_reminder_appointment_processed(appointment.id, success)
                    
                    if success:
                        logger.info(f"Reminder email {appointment.id} sent successfully")
                    else:
                        logger.error(f"Reminder email {appointment.id} failed to send")
                    
                    # Small delay between emails
                    time.sleep(1)
            else:
                logger.info("No reminder appointments found")
            
            logger.info("Cron job execution completed")
            
        except Exception as e:
            logger.error(f"Cron job execution failed: {e}")
    
    def send_logistics_coordination_email(self, appointment: AppointmentScheduling) -> bool:
        """Send email for a specific logistics coordination request"""
        try:
            # Create email content for logistics coordination
            subject = f"Shipping Slot Request - POs: {appointment.po_ids if appointment.po_ids else 'N/A'}"
            body = f"""Hello Team,

We need a shipping slot for POs: {appointment.po_ids if appointment.po_ids else 'N/A'}.
Our required TAT is: {appointment.schedule_datetime.strftime('%B %d, %Y at %I:%M %p')}.

Please provide your earliest available slot and capacity details.

Best regards,
Openleaf Logistics Team"""
            
            # Create message
            message = MIMEMultipart()
            message['to'] = appointment.email_id
            message['from'] = self.sender_email
            message['subject'] = subject
            
            # Add CC recipients if available
            if appointment.cc_email_id:
                message['cc'] = appointment.cc_email_id
            
            # Add body
            message.attach(MIMEText(body, 'plain'))
            
            # Encode message
            raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
            
            # Send message
            result = self.gmail_service.users().messages().send(
                userId='me',
                body={'raw': raw_message}
            ).execute()
            
            message_id = result['id']
            thread_id = result['threadId']
            
            # Get the sent message to extract the actual RFC-2822 Message-ID
            sent_message = self.gmail_service.users().messages().get(
                userId='me',
                id=message_id,
                format='full'
            ).execute()
            
            # Extract the actual RFC-2822 Message-ID from headers
            headers = {h['name'].lower(): h['value'] for h in sent_message['payload'].get('headers', [])}
            rfc_message_id = headers.get('message-id', '')
            
            logger.info(f"Sent logistics coordination email RFC Message-ID: {rfc_message_id}")
            
            # Store sent email for tracking in appointment_email_details table
            sent_email = SentEmail(
                campaign_id=f"logistics_request_{appointment.id}",
                message_id=message_id,
                thread_id=thread_id,
                recipient_email=appointment.email_id,
                recipient_name="Logistics Team",  # Removed channel reference
                subject=subject,
                body=body,
                sent_at=datetime.now(),
                references_chain=None,  # First email has no references
                rfc_message_id=rfc_message_id,  # Store the actual RFC-2822 Message-ID
                user_id=appointment.user_id,
                order_id=appointment.order_id,  # Include order_id from appointment
                record_date=datetime.now(),
                cc_emails=appointment.cc_email_id # Store CC emails
            )
            
            self.database_manager.store_sent_email(sent_email)
            
            logger.info(f"Logistics coordination email sent to {appointment.email_id}: {message_id}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to send logistics coordination email: {e}")
            return False
    
    def send_carrier_email(self, appointment: AppointmentScheduling) -> bool:
        """Send carrier email for phase 2 appointments"""
        try:
            if not appointment.order_id:
                logger.error("No order ID found for phase 2 appointment")
                return False

            # Fetch order details from orders table
            conn = self.database_manager._get_connection()
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            cursor.execute("""
                SELECT lr_number, total_cartons, customer_address
                FROM orders 
                WHERE order_id = %s
            """, (appointment.order_id,))
            
            order_details = cursor.fetchone()
            if not order_details:
                logger.error(f"Order details not found for order ID: {appointment.order_id}")
                return False

            # Format appointment date
            appointment_date = appointment.schedule_datetime.strftime("%d-%m-%Y %H:%M")
            
            # Create email content
            subject = f"Pickup Schedule for Order {order_details['lr_number']}"
            body = f"""
            Hello Team,

            This is to inform you about the delivery schedule for the following order:

            LR Number: {order_details['lr_number']}
            Total Cartons: {order_details['total_cartons']}
            Appointment Date & Time: {appointment_date}
            Delivery Address: {order_details['customer_address']}


            Best regards,
            Openleaf Team
                        """

            # Create message
            message = MIMEMultipart()
            message['to'] = appointment.carrier_email_id
            message['from'] = self.sender_email
            message['subject'] = subject
            
            # Add CC recipients if available
            if appointment.carrier_cc_email_id:
                message['cc'] = appointment.carrier_cc_email_id
            
            # Add body
            message.attach(MIMEText(body, 'plain'))
            
            # Encode message
            raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
            
            # Send message
            result = self.gmail_service.users().messages().send(
                userId='me',
                body={'raw': raw_message}
            ).execute()
            
            message_id = result['id']
            thread_id = result['threadId']
            
            # Get the sent message to extract the actual RFC-2822 Message-ID
            sent_message = self.gmail_service.users().messages().get(
                userId='me',
                id=message_id,
                format='full'
            ).execute()
            
            # Extract the actual RFC-2822 Message-ID from headers
            headers = {h['name'].lower(): h['value'] for h in sent_message['payload'].get('headers', [])}
            rfc_message_id = headers.get('message-id', '')
            
            logger.info(f"Sent carrier email RFC Message-ID: {rfc_message_id}")
            
            # Store sent email details
            sent_email = SentEmail(
                campaign_id=f"phase2_{appointment.order_id}",
                message_id=message_id,
                thread_id=thread_id,
                recipient_email=appointment.carrier_email_id,
                recipient_name="Carrier",
                subject=subject,
                body=body,
                sent_at=datetime.now(),
                replied=False,
                response_sent=False,
                references_chain=None,  # First email has no references
                rfc_message_id=rfc_message_id,  # Store the actual RFC-2822 Message-ID
                user_id=appointment.user_id,
                order_id=appointment.order_id,
                record_date=datetime.now(),
                cc_emails=appointment.carrier_cc_email_id
            )
            
            self.database_manager.store_sent_email(sent_email)
            
            logger.info(f"Phase 2 carrier email sent for appointment {appointment.id}")
            return True
            
        except Exception as e:
            logger.error(f"Error sending phase 2 carrier email: {e}")
            if appointment.id:
                self.database_manager.mark_phase2_appointment_processed(appointment.id, False)
            return False
    
    def send_reminder_email(self, appointment: AppointmentScheduling) -> bool:
        """Send reminder email for appointments"""
        try:
            if not appointment.order_id:
                logger.error("No order ID found for reminder appointment")
                return False

            # Fetch order details from orders table
            conn = self.database_manager._get_connection()
            cursor = conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor)
            
            cursor.execute("""
                SELECT lr_number, total_cartons
                FROM orders 
                WHERE order_id = %s
            """, (appointment.order_id,))
            
            order_details = cursor.fetchone()
            if not order_details:
                logger.error(f"Order details not found for order ID: {appointment.order_id}")
                return False

            # Format appointment date
            appointment_date = appointment.schedule_datetime.strftime("%d-%m-%Y %H:%M")
            
            # Create email content
            subject = f"Reminder: Pickup Schedule for Order {appointment.order_id} Tomorrow"
            body = f"""
Dear Carrier,

This is a reminder that you are scheduled to pick up the following order tomorrow:

Order ID: {appointment.order_id}
LR Number: {order_details['lr_number']}
Total Cartons: {order_details['total_cartons']}
Pickup Date & Time: {appointment_date}

Please ensure you arrive at the scheduled time for pickup.

Best regards,
Openleaf Team
            """

            # Create message
            message = MIMEMultipart()
            message['to'] = appointment.carrier_email_id
            message['from'] = self.sender_email
            message['subject'] = subject
            
            # Add CC recipients if available
            if appointment.carrier_cc_email_id:
                message['cc'] = appointment.carrier_cc_email_id
            
            # Add body
            message.attach(MIMEText(body, 'plain'))
            
            # Encode message
            raw_message = base64.urlsafe_b64encode(message.as_bytes()).decode()
            
            # Send message
            result = self.gmail_service.users().messages().send(
                userId='me',
                body={'raw': raw_message}
            ).execute()
            
            message_id = result['id']
            thread_id = result['threadId']
            
            # Get the sent message to extract the actual RFC-2822 Message-ID
            sent_message = self.gmail_service.users().messages().get(
                userId='me',
                id=message_id,
                format='full'
            ).execute()
            
            # Extract the actual RFC-2822 Message-ID from headers
            headers = {h['name'].lower(): h['value'] for h in sent_message['payload'].get('headers', [])}
            rfc_message_id = headers.get('message-id', '')
            
            logger.info(f"Sent reminder email RFC Message-ID: {rfc_message_id}")
            
            # Store sent email details
            sent_email = SentEmail(
                campaign_id=f"reminder_{appointment.order_id}",
                message_id=message_id,
                thread_id=thread_id,
                recipient_email=appointment.carrier_email_id,
                recipient_name="Carrier",
                subject=subject,
                body=body,
                sent_at=datetime.now(),
                replied=False,
                response_sent=False,
                references_chain=None,  # First email has no references
                rfc_message_id=rfc_message_id,  # Store the actual RFC-2822 Message-ID
                user_id=appointment.user_id,
                order_id=appointment.order_id,
                record_date=datetime.now(),
                cc_emails=appointment.carrier_cc_email_id
            )
            
            self.database_manager.store_sent_email(sent_email)
            
            logger.info(f"Reminder email sent for appointment {appointment.id}")
            return True
            
        except Exception as e:
            logger.error(f"Error sending reminder email: {e}")
            if appointment.id:
                self.database_manager.mark_reminder_appointment_processed(appointment.id, False)
            return False
 
class ProductionGmailAutomation:
    """Main production Gmail automation system with PostgreSQL"""
    
    def __init__(self):
        self.gmail_service = None
        self.database_manager = PostgreSQLDatabaseManager()
        self.push_handler = None
        self.ai_responder = None
        self.cron_sender = None
        
        # Flask app for webhook handling
        self.app = Flask(__name__)
        self.setup_webhook_routes()
        
        # Initialize components
        self.initialize_gmail_service()
        self.initialize_components()
    
    def initialize_gmail_service(self):
        """Initialize Gmail API service"""
        try:
            token_file = os.getenv('GMAIL_TOKEN_FILE', 'server_token.json')
            
            if not os.path.exists(token_file):
                raise FileNotFoundError(f"Token file not found: {token_file}")
            
            creds = Credentials.from_authorized_user_file(token_file)
            
            # Refresh token if needed
            if not creds or not creds.valid:
                if creds and creds.expired and creds.refresh_token:
                    creds.refresh(Request())
                    with open(token_file, 'w') as token:
                        token.write(creds.to_json())
                else:
                    raise Exception("Invalid credentials")
            
            self.gmail_service = build('gmail', 'v1', credentials=creds)
            logger.info("Gmail service initialized successfully")
            
        except Exception as e:
            logger.error(f"Failed to initialize Gmail service: {e}")
            raise
    
    def initialize_components(self):
        """Initialize all system components"""
        self.ai_responder = AIResponseGenerator(self.gmail_service, self.database_manager)
        self.push_handler = GmailPushHandler(self.gmail_service, self.database_manager, self.ai_responder)
        self.cron_sender = EmailCronSender(self.gmail_service, self.database_manager)
        
        # Set up push notifications
        self.push_handler.setup_push_notifications()
        
        logger.info("All components initialized successfully")
    
    def setup_webhook_routes(self):
        """Set up Flask webhook routes"""
        
        @self.app.route('/webhook/gmail', methods=['POST'])
        def handle_gmail_webhook():
            """Handle incoming Gmail push notifications"""
            try:
                notification_data = request.get_json()
                logger.info("Webhook notification received")
                
                # Process notification in background thread
                threading.Thread(
                    target=self.push_handler.handle_push_notification,
                    args=(notification_data,),
                    daemon=True
                ).start()
                
                return jsonify({'status': 'success'}), 200
                
            except Exception as e:
                logger.error(f"Webhook error: {e}")
                return jsonify({'status': 'error', 'message': str(e)}), 500
        
        @self.app.route('/health', methods=['GET'])
        def health_check():
            """Health check endpoint"""
            return jsonify({
                'status': 'healthy',
                'timestamp': datetime.now().isoformat(),
                'components': {
                    'gmail_service': self.gmail_service is not None,
                    'database': True,  # PostgreSQL connection tested in init
                    'ai_responder': self.ai_responder is not None,
                    'push_handler': self.push_handler is not None
                }
            }), 200
        
        @self.app.route('/metrics', methods=['GET'])
        def get_metrics():
            """Get system metrics"""
            try:
                # You can add more metrics here
                return jsonify({
                    'status': 'success',
                    'database': 'postgresql',
                    'timestamp': datetime.now().isoformat()
                }), 200
            except Exception as e:
                return jsonify({'status': 'error', 'message': str(e)}), 500
    
    def add_test_appointment(self):
        """Add test appointment for demonstration with proper data types"""
        try:
            conn = self.database_manager._get_connection()
            cursor = conn.cursor()
            
            # Generate proper UUIDs for test data
            test_user_id = str(uuid.uuid4())
            test_order_id = str(uuid.uuid4())
            test_schedule_time = datetime.now() - timedelta(minutes=5)
            test_po_ids = "PO123,PO456"
            test_cc_emails = "omkar.kamble@openleaf.tech,ajinkya@openleaf.tech,omkar@openleaf.tech"
            
            cursor.execute("""
                INSERT INTO appointments_scheduling (user_id, schedule_datetime, email_id, order_id, is_processed, po_ids, cc_email_id, phase)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                RETURNING id
            """, (
                test_user_id,
                test_schedule_time,
                'omkar.kamble@openleaf.tech',
                test_order_id,
                'N',
                test_po_ids,
                test_cc_emails,
                1  # Phase 1 for regular logistics
            ))
            
            appointment_id = cursor.fetchone()[0]
            conn.commit()
            cursor.close()
            self.database_manager._return_connection(conn)
            
            logger.info(f"Test appointment created with ID: {appointment_id}")
            logger.info(f"User ID: {test_user_id}")
            logger.info(f"Order ID: {test_order_id}")
            logger.info(f"Scheduled for: {test_schedule_time}")
            
        except Exception as e:
            logger.error(f"Failed to create test appointment: {e}")
            if 'conn' in locals():
                conn.rollback()
                self.database_manager._return_connection(conn)
    
    def add_test_phase2_appointment(self):
        """Add test phase 2 appointment for carrier email demonstration"""
        try:
            conn = self.database_manager._get_connection()
            cursor = conn.cursor()
            
            # Generate proper UUIDs for test data
            test_user_id = str(uuid.uuid4())
            test_order_id = str(uuid.uuid4())
            test_schedule_time = datetime.now() - timedelta(minutes=5)
            test_po_ids = "PO789,PO101"  # For appointments_scheduling
            test_po_ids_json = json.dumps(["PO789", "PO101"])  # For orders table
            test_cc_emails = "ajinkya.naik@sakec.ac.in"
            test_carrier_email = "omkar.kamble@openleaf.tech,omkar@openleaf.tech,ajinkya@openleaf.tech"  # Test carrier email
            test_carrier_cc = "ajinkya.naik@sakec.ac.in"  # Test carrier CC
            
            # First insert test order data with all required fields
            cursor.execute("""
                INSERT INTO orders (
                    order_id, user_id, original_order_id, po_id, warehouse_id,
                    customer_name, customer_address, customer_phone, customer_pincode,
                    customer_city, customer_state, customer_email, truck_load_type,
                    order_type, order_mode, lr_number, total_cartons
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                )
            """, (
                test_order_id,
                test_user_id,  # Required user_id
                f"WEB-{test_order_id[:8]}",  # Original order ID
                test_po_ids_json,  # PO IDs as JSON array
                str(uuid.uuid4()),  # Warehouse ID
                "Test Customer",  # Customer name
                "123 Test Street, Mumbai, Maharashtra, 400001",  # Customer address
                "+919876543210",  # Customer phone
                "400001",  # Customer pincode
                "Mumbai",  # Customer city
                "Maharashtra",  # Customer state
                "test@example.com",  # Customer email
                "FTL",  # Truck load type
                "Delivery",  # Order type
                "Surface",  # Order mode
                "LR123456",  # Test LR number
                25  # Test carton count
            ))
            
            # Then insert appointment
            cursor.execute("""
                INSERT INTO appointments_scheduling (user_id, schedule_datetime, email_id, order_id, is_processed, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                RETURNING id
            """, (
                test_user_id,
                test_schedule_time,
                'logistics@example.com',  # Original logistics email
                test_order_id,
                'N',  # Not processed initially
                test_po_ids,  # Keep as comma-separated string for appointments
                test_cc_emails,
                2,  # Phase 2 for carrier emails
                test_carrier_email,
                test_carrier_cc
            ))
            
            appointment_id = cursor.fetchone()[0]
            conn.commit()
            
            logger.info(f"Created test phase 2 appointment {appointment_id} with order {test_order_id}")
            logger.info(f"Carrier Email: {test_carrier_email}")
            logger.info(f"CC Email: {test_carrier_cc}")
            
            return appointment_id
            
        except Exception as e:
            logger.error(f"Error creating test phase 2 appointment: {e}")
            if conn:
                conn.rollback()
            return None
        finally:
            if conn:
                self.database_manager._return_connection(conn)
    
    def add_test_reminder_appointment(self):
        """Add test reminder appointment with order_activity record"""
        try:
            conn = self.database_manager._get_connection()
            cursor = conn.cursor()
            
            # Generate proper UUIDs for test data
            test_user_id = str(uuid.uuid4())
            test_order_id = str(uuid.uuid4())
            test_schedule_time = datetime.now() - timedelta(minutes=5)  # Recent past for cron pickup
            test_po_ids = "PO999,PO101"
            test_po_ids_json = json.dumps(["PO999", "PO101"])  # For orders table
            test_cc_emails = "ajinkya.naik@sakec.ac.in"
            test_carrier_email = "omkar.kamble@openleaf.tech"  # Test carrier email
            test_carrier_cc = "ajinkya@openleaf.tech, omkar@openleaf.tech, nikhil@openleaf.tech, sahil@openleaf.tech, shreyash@openleaf.tech, abhay@openleaf.tech, richard@openleaf.tech"  # Test carrier CC
            confirmed_time = datetime.now() + timedelta(hours=2)  # 2 hours from now
            
            # First create the order record
            cursor.execute("""
                INSERT INTO orders (
                    order_id, user_id, original_order_id, po_id, warehouse_id,
                    customer_name, customer_address, customer_phone, customer_pincode,
                    customer_city, customer_state, customer_email, truck_load_type,
                    order_type, order_mode, lr_number, total_cartons
                ) VALUES (
                    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
                )
            """, (
                test_order_id,
                test_user_id,  # Required user_id
                f"WEB-{test_order_id[:8]}",  # Original order ID
                test_po_ids_json,  # PO IDs as JSON array
                str(uuid.uuid4()),  # Warehouse ID
                "Test Customer",  # Customer name
                "123 Test Street",  # Customer address
                "+919876543210",  # Customer phone
                "400001",  # Customer pincode
                "Mumbai",  # Customer city
                "Maharashtra",  # Customer state
                "test@example.com",  # Customer email
                "FTL",  # Truck load type
                "Delivery",  # Order type
                "Surface",  # Order mode
                "LR123456",  # Test LR number
                25  # Test carton count
            ))
            
            # Then create the order_activity record
            cursor.execute("""
                INSERT INTO order_activity (order_id, is_appointment_confirmed, appointment_scheduled_at)
                VALUES (%s, %s, %s)
            """, (test_order_id, True, confirmed_time))
            
            # Finally create the reminder appointment
            cursor.execute("""
                INSERT INTO appointments_scheduling (user_id, schedule_datetime, email_id, order_id, is_processed, po_ids, cc_email_id, phase, carrier_email_id, carrier_cc_email_id, is_reminder)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                RETURNING id
            """, (
                test_user_id,
                test_schedule_time,
                'logistics@example.com',  # Original logistics email
                test_order_id,
                'N',  # Not processed initially (changed from 'Y' to 'N')
                test_po_ids,
                test_cc_emails,
                1,  # Phase 1 but with reminder flag
                test_carrier_email,
                test_carrier_cc,
                True  # is_reminder = TRUE
            ))
            
            appointment_id = cursor.fetchone()[0]
            conn.commit()
            cursor.close()
            self.database_manager._return_connection(conn)
            
            logger.info(f"Test reminder appointment created with ID: {appointment_id}")
            logger.info(f"User ID: {test_user_id}")
            logger.info(f"Order ID: {test_order_id}")
            logger.info(f"Carrier Email: {test_carrier_email}")
            logger.info(f"Confirmed Time: {confirmed_time}")
            logger.info(f"Scheduled for: {test_schedule_time}")
            
        except Exception as e:
            logger.error(f"Failed to create test reminder appointment: {e}")
            if 'conn' in locals():
                conn.rollback()
                self.database_manager._return_connection(conn)
    
    def run_cron_job(self):
        """Execute cron job"""
        logger.info("Executing cron job")
        self.cron_sender.run_cron_job()
    
    def start_webhook_server(self, host='0.0.0.0', port=5000):
        """Start webhook server"""
        logger.info(f"Starting webhook server on {host}:{port}")
        self.app.run(host=host, port=port, debug=False)

def main():
    """Main function for different execution modes"""
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python production_gmail_automation_postgres.py [cron|webhook|test|test-phase2|test-reminder|test-all]")
        print("  cron          - Run cron job to send emails from appointments_scheduling table")
        print("  webhook       - Start webhook server for push notifications")
        print("  test          - Add test phase 1 appointment and run cron")
        print("  test-phase2   - Add test phase 2 appointment and run cron")
        print("  test-reminder - Add test reminder appointment and run cron")
        print("  test-all      - Add all test appointments and run cron")
        return
    
    mode = sys.argv[1]
    
    try:
        automation = ProductionGmailAutomation()
        
        if mode == 'cron':
            automation.run_cron_job()
            
        elif mode == 'webhook':
            automation.start_webhook_server()
            
        elif mode == 'test':
            print("Test mode: Adding test phase 1 appointment and running cron")
            automation.add_test_appointment()
            automation.run_cron_job()
            print("Test phase 1 appointment created and cron job executed")
            print("Now start webhook server to handle replies: python production_gmail_automation_postgres.py webhook")
            
        elif mode == 'test-phase2':
            print("Test mode: Adding test phase 2 appointment and running cron")
            automation.add_test_phase2_appointment()
            automation.run_cron_job()
            print("Test phase 2 appointment created and cron job executed")
            print("Now start webhook server to handle replies: python production_gmail_automation_postgres.py webhook")
            
        elif mode == 'test-reminder':
            print("Test mode: Adding test reminder appointment and running cron")
            automation.add_test_reminder_appointment()
            automation.run_cron_job()
            print("Test reminder appointment created and cron job executed")
            print("Now start webhook server to handle replies: python production_gmail_automation_postgres.py webhook")
            
        elif mode == 'test-all':
            print("Test mode: Adding all test appointments and running cron")
            automation.add_test_appointment()
            automation.add_test_phase2_appointment()
            automation.add_test_reminder_appointment()
            automation.run_cron_job()
            print("All test appointments created and cron job executed")
            print("Now start webhook server to handle replies: python production_gmail_automation_postgres.py webhook")
            
        else:
            print(f"Unknown mode: {mode}")
    
    except Exception as e:
        logger.error(f"Application failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main() 
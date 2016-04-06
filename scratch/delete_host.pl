#!/usr/bin/perl

use strict; 
use XML::Simple; 
use Data::Dumper; 
use Data::Validate::IP qw (is_private_ipv4) ; 
use Net::DNS ; 
use Net::Ping ; 
use YAML::Syck; 
use PMC; 
   

my $PMCDIR = '/var/local/pmc'; 
my $HOST_IMAGE_LOCATION = '/var/lib/libvirt/images'; 

my $hostname = shift; 
my $ip = shift; 

die __FILE__." <hostname> <ip>\n" if !(defined $ip && defined $hostname) ; 



my $ipf = PMC::VerifyValidInputForDelete($hostname,$ip) ; 
die if(!$ipf) ;
PMC::DeleteAndDestroyVM($hostname) ; 
PMC::DeleteHostImages($hostname) ; 
PMC::DeleteHostDnsDhcp($hostname,$ip) ; 

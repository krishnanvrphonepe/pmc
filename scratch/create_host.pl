#!/usr/bin/perl

use strict; 
use XML::Simple; 
use Data::Dumper; 
use Data::Validate::IP qw (is_private_ipv4) ; 
use Net::DNS ; 
use Net::Ping ; 
use YAML::Syck; 
use PMC; 
   


my $hostname = shift; 
my $ip = shift; 
my $size = shift; 
$size = 'C1M2' if(!defined $size) ; 
my $sizef =PMC::VerifyValidSize($size) ; 

die __FILE__." <hostname> <ip>\n" if !(defined $ip && defined $hostname) ; 



my $ipf = PMC::VerifyValidIP($ip) ; 
die if(!$ipf) ;

my $hostnamef = PMC::VerifyValidHost($hostname) ; 
die if(!$hostnamef) ;

die if(!$sizef) ; 



PMC::GenerateNetworkConfig($hostname,$ip) ; 
PMC::GenerateCloudInitConfig($hostname) ; 
PMC::GenerateLibvirtXML($hostname,$ip,$size) ; 
PMC::DefineAndStartVM($hostname) ; 

